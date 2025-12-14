package champ2

import (
	"fmt"
	"hallSlot/common/protocol"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// Redis频道常量
const (
	GameResultChannelPattern = "champ:game_result:%d:%d" // 对局结果频道格式（游戏层发布给hallSlot）
	GameStartChannelPattern  = "champ:game_start:%d:%d"  // 游戏开始频道格式（hallSlot发布给游戏层）
)

// GetGameResultChannel 生成对局结果频道名
func GetGameResultChannel(gameId, roomId int32) string {
	return fmt.Sprintf(GameResultChannelPattern, gameId, roomId)
}

// GetGameStartChannel 生成游戏开始频道名
func GetGameStartChannel(gameId, roomId int32) string {
	return fmt.Sprintf(GameStartChannelPattern, gameId, roomId)
}

/*************** Redis Subscriber ***************/

type RedisSubscriber struct {
	champ  *Champ
	client *redis.Client
	stopCh chan struct{}
}

func NewRedisSubscriber(client *redis.Client, champ *Champ) *RedisSubscriber {
	return &RedisSubscriber{
		champ:  champ,
		client: client,
		stopCh: make(chan struct{}),
	}
}

func (r *RedisSubscriber) Start() error {
	channel := GetGameResultChannel(r.champ.cfg.GameId, r.champ.cfg.RoomId)
	pubsub := r.client.Subscribe(channel)
	if _, err := pubsub.Receive(); err != nil {
		return err
	}
	log.Debugf("Subscribed to %s", channel)

	go func() {
		defer pubsub.Close()
		for {
			select {
			case <-r.stopCh:
				return
			case msg := <-pubsub.Channel():
				r.handleGameResult(msg.Payload)
			}
		}
	}()
	return nil
}

func (r *RedisSubscriber) Stop() { close(r.stopCh) }

func (r *RedisSubscriber) handleGameResult(payload string) {
	var result protocol.ChampGameResult
	if err := proto.Unmarshal([]byte(payload), &result); err != nil {
		log.Errorf("[champ] unmarshal game result failed: %v", err)
		return
	}

	if result.GetGameId() != r.champ.cfg.GameId || result.GetRoomId() != r.champ.cfg.RoomId {
		log.Warn("[champ] gameId/roomId mismatch")
		return
	}

	if _, ok := r.champ.cfg.Stages[int32(result.GetStageId())]; !ok {
		log.Warnf("[champ] unknown stageId=%d", result.GetStageId())
		return
	}

	for _, p := range result.GetPlayers() {
		if err := r.applyPlayerResult(&result, p); err != nil {
			log.Errorf("[champ] apply result uid=%d err=%v", p.GetUid(), err)
		}
	}
}

/*************** Core Apply Logic ***************/

func (r *RedisSubscriber) applyPlayerResult(gameResult *protocol.ChampGameResult, playerResult *protocol.ChampPlayerResult) error {

	player, err := r.loadOrCreatePlayer(playerResult.GetUid())
	if err != nil {
		return err
	}

	score, err := r.calcScore(gameResult, playerResult)
	if err != nil {
		return err
	}

	player.Score += score

	switch gameResult.GetStageId() {
	case StageIDQualifying:
		player.Qualifying++
	case StageIDKnockout:
		if playerResult.GetIsWinner() {
			player.WinCount++
		}
	case StageIDFinal:
		// final only score
	}

	if err = r.champ.persistence.SavePlayer(player); err != nil {
		return err
	}

	log.Debugf("[champ] uid=%d stage=%d rank=%d score+%d total=%d",
		player.UID, gameResult.GetStageId(), playerResult.GetRank(), score, player.Score,
	)

	return nil
}

/*************** Helpers ***************/

func (r *RedisSubscriber) loadOrCreatePlayer(uid int64) (*Player, error) {
	// 1. memory
	if p, ok := r.champ.players[uid]; ok {
		return p, nil
	}

	// 2. redis
	p, err := r.champ.persistence.LoadPlayer(uid)
	if err == nil && p != nil {
		r.champ.players[uid] = p
		return p, nil
	}

	// 3. create
	p = &Player{
		UID:      uid,
		JoinedAt: timeNow(),
	}
	r.champ.players[uid] = p
	return p, nil
}

func (r *RedisSubscriber) calcScore(gameResult *protocol.ChampGameResult, playerResult *protocol.ChampPlayerResult) (int64, error) {
	stageCfg := r.champ.cfg.Stages[int32(gameResult.GetStageId())]
	rank := int(playerResult.GetRank()) - 1
	if rank < 0 || rank >= len(stageCfg.Scores) {
		return 0, fmt.Errorf("invalid rank %d", playerResult.GetRank())
	}
	return stageCfg.Scores[rank], nil
}

func timeNow() int64 {
	return time.Now().Unix()
}
