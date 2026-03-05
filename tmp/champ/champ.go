package champv2

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 全局logger
var log *zap.SugaredLogger

func init() {
	zapConfig := zap.NewDevelopmentConfig()
	zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	zapConfig.EncoderConfig.ConsoleSeparator = " "
	zapConfig.EncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + t.Format("2006/01/02 15:04:05.000") + "]")
	}
	zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	zapConfig.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder

	zapLogger, err := zapConfig.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize zap logger: %v", err))
	}
	log = zapLogger.Sugar()
}

type Championship struct {
	GameID     int32
	RoomID     int32
	ID         string
	config     *ChampConfig
	current    ChampState
	listener   Listener
	workerPool *WorkerPool
	scheduler  Scheduler
	ctx        context.Context
	cancel     context.CancelFunc
	runtime    map[ChampState]*PhaseRuntime
	mu         sync.RWMutex
	redis      *redis.Client
	players    map[int64]*Player
}

type PhaseRuntime struct {
	State     int32
	EnteredAt time.Time
	LeaveAt   time.Time
}

type Player struct {
	UID       int64
	Score     int64
	WinCount  int32
	JoinedAt  int64
	TableID   int32
	LastPhase int32
}

func NewChampionship(gameID, roomID int32, config *ChampConfig, redis *redis.Client) *Championship {
	ctx, cancel := context.WithCancel(context.Background())
	wp := NewWorkerPool(8, 64)

	id := fmt.Sprintf("Champ-%v-%d-%d", GetMondayDate(0).Format("20060102"), gameID, roomID)

	c := &Championship{
		GameID:     gameID,
		RoomID:     roomID,
		ID:         id,
		config:     config,
		current:    StateIdle,
		listener:   NewPhaseListener(),
		workerPool: wp,
		scheduler:  NewHeapScheduler(WithHeapContext(ctx), WithWorkerPool(wp)),
		ctx:        ctx,
		cancel:     cancel,
		runtime:    make(map[ChampState]*PhaseRuntime),
		redis:      redis,
		players:    make(map[int64]*Player),
	}

	c.loadRuntimeFromRedis()
	c.scheduleAndAlign()
	c.loadPlayersByStage()

	log.Infof("NewChampionship. id=%q, current=%q", c.ID, c.current)
	return c
}

func (c *Championship) Stop() {
	c.scheduler.Stop()
	c.cancel()
}

func (c *Championship) CurrentState() ChampState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

func (c *Championship) scheduleAndAlign() {
	now := time.Now()
	alignedPhase := StateIdle

	for _, stateID := range c.config.Order {
		p, exists := c.config.Phases[stateID]
		if !exists {
			continue
		}

		// 已过阶段
		if now.After(p.End) {
			continue
		}

		// 当前阶段
		if now.After(p.Start) && now.Before(p.End) {
			c.enterPhase(stateID, now, true)
			alignedPhase = stateID
		}

		// 调度未来 enter
		if now.Before(p.Start) {
			c.scheduler.ScheduleAt(p.Start, func(s ChampState, t time.Time) func() {
				return func() { c.enterPhase(s, t, false) }
			}(stateID, p.Start))
		}

		// 调度未来 leave
		if now.Before(p.End) {
			c.scheduler.ScheduleAt(p.End, func(s ChampState, t time.Time) func() {
				return func() { c.leavePhase(s, t) }
			}(stateID, p.End))
		}

		// 调度 reminders
		for _, sec := range p.Reminders {
			notifyAt := p.Start.Add(-time.Duration(sec) * time.Second)
			if notifyAt.After(now) {
				c.scheduler.ScheduleAt(notifyAt, func(s ChampState, sec int, t time.Time) func() {
					return func() { c.listener.OnReminder(c, s, sec, t) }
				}(stateID, sec, notifyAt))
			}
		}
	}

	c.mu.Lock()
	c.current = alignedPhase
	c.mu.Unlock()
	log.Infof("[Champ] current state aligned: %s", c.current)
}

func (c *Championship) enterPhase(s ChampState, at time.Time, force bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pr := c.runtime[s]
	if !force && pr != nil && !pr.EnteredAt.IsZero() {
		return
	}
	if pr == nil {
		pr = &PhaseRuntime{State: int32(s), EnteredAt: at}
		c.runtime[s] = pr
	} else {
		pr.EnteredAt, pr.State = at, int32(s)
	}
	c.current = s

	c.workerPool.Submit(func() {
		log.Infof("[%s] >>> enter %s", c.ID, s)
		c.saveRuntimeToRedis()
		c.listener.OnEnterPhase(c, s, at)
	})
}

func (c *Championship) leavePhase(s ChampState, at time.Time) {
	c.mu.RLock()
	pr, ok := c.runtime[s]
	if !ok || pr.EnteredAt.IsZero() {
		c.mu.RUnlock()
		return
	}
	duration := at.Sub(pr.EnteredAt)
	if duration < 0 {
		duration = 0
	}
	pr.LeaveAt = at
	c.mu.RUnlock()

	c.workerPool.Submit(func() {
		c.saveRuntimeToRedis()
		c.handlePhaseEnd(s, at, duration)
		c.listener.OnLeavePhase(c, s, at, duration)
		log.Infof("[%s] <<< leave %s (%v)", c.ID, s, duration)
	})
}

func (c *Championship) handlePhaseEnd(s ChampState, at time.Time, duration time.Duration) {
	nextStage := c.config.NextStageByOrder(s)
	if nextStage == StateIdle || nextStage == StateFinished {
		return
	}

	pc := c.getPhaseConfig(nextStage)
	if pc == nil || pc.Keep <= 0 {
		return
	}

	rankKey := c.getRankKey()
	allPlayers, err := c.redis.ZRevRangeWithScores(rankKey, 0, -1).Result()
	if err != nil || len(allPlayers) == 0 {
		return
	}

	keepCount := int(pc.Keep)
	for i, player := range allPlayers {
		uid := strToInt(player.Member.(string))
		isKeep := i < keepCount

		if p, err := c.loadPlayerFromRedis(uid); err == nil {
			if isKeep {
				// 晋级
				p.LastPhase, p.Score = int32(nextStage), 0
				c.players[uid] = p
				c.redis.ZAdd(rankKey, redis.Z{Score: 0, Member: fmt.Sprintf("%d", uid)})
			} else {
				// 淘汰
				p.LastPhase = int32(s)
				delete(c.players, uid)
			}
			_ = c.savePlayerToRedis(uid, p)
		}
	}

	c.redis.ZRemRangeByRank(rankKey, pc.Keep, -1)
}

// ===== Redis & Player Helpers =====
func (c *Championship) getPhaseKey(s ChampState) string { return fmt.Sprintf("champ:%s-%s", c.ID, s) }
func (c *Championship) getRegisterKey() string          { return fmt.Sprintf("champ:%s:register", c.ID) }
func (c *Championship) getRankKey() string              { return fmt.Sprintf("champ:%s:rank", c.ID) }
func (c *Championship) getPlayerKey(uid int64) string {
	return fmt.Sprintf("champ:%s:player:%d", c.ID, uid)
}

func (c *Championship) saveRuntimeToRedis() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	expire := GetMondayDate(1)
	for s, pr := range c.runtime {
		if pr == nil {
			continue
		}
		data := map[string]interface{}{
			"State":     pr.State,
			"EnteredAt": pr.EnteredAt.Unix(),
		}
		if !pr.LeaveAt.IsZero() {
			data["LeaveAt"] = pr.LeaveAt.Unix()
		}
		if err := c.redis.HMSet(c.getPhaseKey(s), data).Err(); err == nil {
			c.redis.ExpireAt(c.getPhaseKey(s), expire)
		}
	}
}

func (c *Championship) loadRuntimeFromRedis() {
	for _, s := range c.config.Order {
		data, err := c.redis.HGetAll(c.getPhaseKey(s)).Result()
		if err != nil || len(data) == 0 {
			continue
		}
		pr := &PhaseRuntime{State: int32(strToInt(data["State"]))}
		if v, ok := data["EnteredAt"]; ok && v != "" {
			pr.EnteredAt = time.Unix(strToInt(v), 0)
		}
		if v, ok := data["LeaveAt"]; ok && v != "" {
			pr.LeaveAt = time.Unix(strToInt(v), 0)
		}
		c.runtime[s] = pr
	}
	c.mu.Lock()
	c.current = c.computeCurrent()
	c.mu.Unlock()
}

func (c *Championship) loadPlayersByStage() {
	c.mu.RLock()
	stage := c.current
	c.mu.RUnlock()

	if pc := c.getPhaseConfig(stage); pc != nil && pc.Keep > 0 {
		if uids, err := c.GetPlayerRankTopN(pc.Keep); err == nil {
			for _, uid := range uids {
				if p, err := c.loadPlayerFromRedis(uid); err == nil {
					c.players[uid] = p
				}
			}
		}
	}
}

func (c *Championship) loadPlayerFromRedis(uid int64) (*Player, error) {
	key := c.getPlayerKey(uid)
	data, err := c.redis.HGetAll(key).Result()
	if err != nil || len(data) == 0 {
		return nil, err
	}
	return &Player{
		UID:       uid,
		Score:     strToInt(data["Score"]),
		WinCount:  int32(strToInt(data["WinCount"])),
		JoinedAt:  strToInt(data["JoinedAt"]),
		TableID:   int32(strToInt(data["TableID"])),
		LastPhase: int32(strToInt(data["LastPhase"])),
	}, nil
}

func (c *Championship) savePlayerToRedis(uid int64, p *Player) error {
	key := c.getPlayerKey(uid)
	data := map[string]interface{}{
		"Score":     p.Score,
		"WinCount":  p.WinCount,
		"JoinedAt":  p.JoinedAt,
		"TableID":   p.TableID,
		"LastPhase": p.LastPhase,
	}
	if err := c.redis.HMSet(key, data).Err(); err != nil {
		return err
	}
	c.redis.ExpireAt(key, GetMondayDate(1))
	return nil
}

func (c *Championship) GetPlayerRankTopN(n int64) ([]int64, error) {
	rankKey := c.getRankKey()
	list, err := c.redis.ZRevRange(rankKey, 0, n-1).Result()
	if err != nil {
		return nil, err
	}
	res := make([]int64, 0, len(list))
	for _, v := range list {
		res = append(res, strToInt(v))
	}
	return res, nil
}

// ===== Utilities =====
func (c *Championship) getPhaseConfig(s ChampState) *PhaseConfig {
	if pc, exists := c.config.Phases[s]; exists {
		return &pc
	}
	return nil
}

func strToInt(s string) int64 { i, _ := strconv.ParseInt(s, 10, 64); return i }

func (c *Championship) computeCurrent() ChampState {
	now := time.Now()
	for i := len(c.config.Order) - 1; i >= 0; i-- {
		s := c.config.Order[i]
		if pr, ok := c.runtime[s]; ok && pr.LeaveAt.IsZero() && now.After(pr.EnteredAt) {
			return s
		}
	}
	return StateIdle
}
