package champv2

import (
	"fmt"
	//"log"
	"time"

	"github.com/go-redis/redis"
)

type Listener interface {
	OnEnterPhase(c *Championship, phase ChampState, at time.Time)
	OnLeavePhase(c *Championship, phase ChampState, at time.Time, duration time.Duration)
	OnReminder(c *Championship, phase ChampState, secondsBefore int, at time.Time)
}

type DefaultListener struct{}

func NewPhaseListener() Listener {
	return &DefaultListener{}
}

func (l *DefaultListener) OnEnterPhase(c *Championship, phase ChampState, at time.Time) {
	switch phase {
	case StateKO16:
		log.Infof("[Listener] Phase ENTER: %q at %s", phase, at.Format("15:04:05"))
	default:
	}
}

func (l *DefaultListener) OnLeavePhase(c *Championship, phase ChampState, at time.Time, duration time.Duration) {
	switch phase {
	case StateKO16:
		log.Infof("[Listener] Phase LEAVE: %q at %s, duration %v", phase, at.Format("15:04:05"), duration)
	default:
	}
}

func (l *DefaultListener) OnReminder(c *Championship, phase ChampState, secondsBefore int, at time.Time) {
	log.Infof("[Listener] Phase Reminder: %d seconds before %q at %s", secondsBefore, phase, at.Format("15:04:05"))
}

// RegisterUser 玩家报名
func (c *Championship) RegisterUser(uid int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.current != StateRegistering {
		return fmt.Errorf("cannot register, current state is %s, uid=%d", c.current, uid)
	}

	if _, exists := c.players[uid]; exists {
		return fmt.Errorf("player %d already registered", uid)
	}

	// 检查注册人数限制
	pc := c.getPhaseConfig(StateRegistering)
	if pc != nil && pc.Keep > 0 && int64(len(c.players)) >= pc.Keep {
		return fmt.Errorf("registration full, max %d players", pc.Keep)
	}

	uidStr := fmt.Sprintf("%d", uid)
	registerKey := c.getRegisterKey()
	rankKey := c.getRankKey()

	p := &Player{
		UID:       uid,
		Score:     0,
		WinCount:  0,
		JoinedAt:  time.Now().Unix(),
		LastPhase: int32(StateRegistering),
	}

	// 写入报名 - 用 List 记录报名顺序
	if _, err := c.redis.LPush(registerKey, uidStr).Result(); err != nil {
		return fmt.Errorf("LPush register error: %v", err)
	}
	// 写入排行
	if _, err := c.redis.ZAdd(rankKey, redis.Z{Score: 0, Member: uidStr}).Result(); err != nil {
		return fmt.Errorf("ZAdd rank error: %v", err)
	}
	// 写入玩家数据
	if err := c.savePlayerToRedis(uid, p); err != nil {
		return fmt.Errorf("save player error: %v", err)
	}

	c.players[uid] = p
	expireAt := GetMondayDate(1)
	c.redis.ExpireAt(registerKey, expireAt)
	c.redis.ExpireAt(rankKey, expireAt)

	log.Infof("User %d registered to championship %s", uid, c.ID)
	return nil
}
