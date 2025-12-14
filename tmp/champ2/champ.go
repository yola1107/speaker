package champ2

import (
	"context"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

type ChampNotifyCallback func(players []*Player, tables [][]int64, stageID int32, round int32, clientTableId int32)

type Champ struct {
	ctx                context.Context
	cancel             context.CancelFunc
	cfg                *ChampConfig
	subscriber         *RedisSubscriber
	persistence        *RedisPersistence
	fsm                *FSM
	onEnterTableNotify ChampNotifyCallback
	players            map[int64]*Player
}

func NewChamp(cfg *ChampConfig, redisClient *redis.Client) *Champ {
	if cfg == nil {
		cfg = GetDefaultChampConfig(118, 200)
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Champ{
		ctx:     ctx,
		cancel:  cancel,
		cfg:     cfg,
		players: make(map[int64]*Player),
	}
	c.persistence = NewRedisPersistence(redisClient, cfg.GameId, cfg.RoomId)
	c.subscriber = NewRedisSubscriber(redisClient, c)
	c.fsm = NewFSM(c.persistence, cfg)
	return c
}

func (c *Champ) Start() {
	c.fsm.Start()
	c.loadPlayersByStage()
	c.subscriber.Start()
	log.Infof("Championship started: gameID=%d roomID=%d", c.cfg.GameId, c.cfg.RoomId)
}

func (c *Champ) Stop() {
	c.fsm.Stop()
	c.subscriber.Stop()
	c.cancel()
	log.Infof("Championship stopped: gameID=%d roomID=%d", c.cfg.GameId, c.cfg.RoomId)
}

// 根据当前 FSM 状态加载玩家
func (c *Champ) loadPlayersByStage() {
	stage := c.fsm.State()
	var uids []int64
	switch stage {
	case StateQualifying:
		// 全量
		uids, _ = c.persistence.TopN(10000)
	case StateKnockout64:
		uids, _ = c.persistence.TopN(64)
	case StateKnockout32:
		uids, _ = c.persistence.TopN(32)
	case StateKnockout16:
		uids, _ = c.persistence.TopN(16)
	case StateKnockout8:
		uids, _ = c.persistence.TopN(8)
	case StateFinal:
		uids, _ = c.persistence.TopN(8)
	}
	for _, uid := range uids {
		data, err := c.persistence.LoadPlayer(uid)
		if err != nil {
			log.Errorf("load player failed, uid=%d, err=%v", uid, err)
			continue
		}
		c.players[uid] = data
	}
}
