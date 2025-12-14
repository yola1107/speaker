package champ2

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

const (
	metaKeyFmt   = "champ:meta:%d:%d"
	playerKeyFmt = "champ:player:%d:%d:%d"
	rankKeyFmt   = "champ:rank:%d:%d"
	defaultTTL   = 7 * 24 * time.Hour
)

type ChampMeta struct {
	GameID int32
	RoomID int32
	State  int32
}

// Player 玩家
type Player struct {
	UID        int64 // 玩家ID
	Score      int64 // 当前积分
	WinCount   int32 // 淘汰赛胜场数
	Qualifying int32 // 海选赛完成的局数（1-5）
	JoinedAt   int64 // 加入时间
	Eliminated int64 // 淘汰时间（零值表示未淘汰）
	TableID    int32 //
}

type RedisPersistence struct {
	rdb  *redis.Client
	game int32
	room int32
	ttl  time.Duration
}

func NewRedisPersistence(rdb *redis.Client, gameID, roomID int32) *RedisPersistence {
	return &RedisPersistence{rdb: rdb, game: gameID, room: roomID, ttl: defaultTTL}
}

func (p *RedisPersistence) metaKey() string { return fmt.Sprintf(metaKeyFmt, p.game, p.room) }
func (p *RedisPersistence) rankKey() string { return fmt.Sprintf(rankKeyFmt, p.game, p.room) }
func (p *RedisPersistence) playerKey(uid int64) string {
	return fmt.Sprintf(playerKeyFmt, p.game, p.room, uid)
}

func (p *RedisPersistence) LoadMeta() (*ChampMeta, error) {
	vals, err := p.rdb.HGetAll(p.metaKey()).Result()
	if err != nil || len(vals) == 0 {
		return nil, err
	}
	return &ChampMeta{
		GameID: p.game,
		RoomID: p.room,
		State:  int32(strToInt(vals["state"])),
	}, nil
}

func (p *RedisPersistence) SaveMeta(state int32) error {
	pipe := p.rdb.TxPipeline()
	pipe.HMSet(p.metaKey(), map[string]interface{}{
		"state":      state,
		"updated_at": time.Now().Unix(),
	})
	pipe.Expire(p.metaKey(), p.ttl)
	_, err := pipe.Exec()
	return err
}

func (p *RedisPersistence) SavePlayer(pl *Player) error {
	key := p.playerKey(pl.UID)
	pipe := p.rdb.TxPipeline()
	pipe.HMSet(key, map[string]interface{}{
		"uid":        pl.UID,
		"score":      pl.Score,
		"win":        pl.WinCount,
		"qual":       pl.Qualifying,
		"joined":     pl.JoinedAt,
		"eliminated": pl.Eliminated,
		"table":      pl.TableID,
	})
	pipe.Expire(key, p.ttl)
	_, err := pipe.Exec()
	return err
}

func (p *RedisPersistence) LoadPlayer(uid int64) (*Player, error) {
	vals, err := p.rdb.HGetAll(p.playerKey(uid)).Result()
	if err != nil || len(vals) == 0 {
		return nil, err
	}
	return &Player{
		UID:        uid,
		Score:      strToInt(vals["score"]),
		WinCount:   int32(strToInt(vals["win"])),
		Qualifying: int32(strToInt(vals["qual"])),
		JoinedAt:   strToInt(vals["joined"]),
		Eliminated: strToInt(vals["eliminated"]),
		TableID:    int32(strToInt(vals["table"])),
	}, nil
}

func (p *RedisPersistence) IncrScore(uid int64, delta int64) error {
	pipe := p.rdb.TxPipeline()
	pipe.ZIncrBy(p.rankKey(), float64(delta), fmt.Sprintf("%d", uid))
	pipe.Expire(p.rankKey(), p.ttl)
	_, err := pipe.Exec()
	return err
}

func (p *RedisPersistence) TopN(n int64) ([]int64, error) {
	list, err := p.rdb.ZRevRange(p.rankKey(), 0, n-1).Result()
	if err != nil {
		return nil, err
	}
	res := make([]int64, 0, len(list))
	for _, v := range list {
		id, _ := strconv.ParseInt(v, 10, 64)
		res = append(res, id)
	}
	return res, nil
}

// PromoteCAS 推进阶段并裁掉多余玩家
func (p *RedisPersistence) PromoteCAS(expectState int32, nextState int32, keep int64) ([]int64, error) {
	var winners []int64
	err := p.rdb.Watch(func(tx *redis.Tx) error {
		cur, err := tx.HGet(p.metaKey(), "state").Int64()
		if err != nil {
			return err
		}
		if int32(cur) != expectState {
			return redis.TxFailedErr
		}
		top, err := tx.ZRevRange(p.rankKey(), 0, keep-1).Result()
		if err != nil {
			return err
		}
		for _, v := range top {
			id, _ := strconv.ParseInt(v, 10, 64)
			winners = append(winners, id)
		}
		pipe := tx.TxPipeline()
		pipe.HMSet(p.metaKey(), map[string]interface{}{
			"state":      nextState,
			"updated_at": time.Now().Unix(),
		})
		pipe.ZRemRangeByRank(p.rankKey(), keep, -1)
		_, err = pipe.Exec()
		return err
	}, p.metaKey(), p.rankKey())

	if errors.Is(err, redis.TxFailedErr) {
		log.Warn("state mismatch, promote skipped")
		return nil, nil
	}
	return winners, err
}

func (p *RedisPersistence) Promote(ctx context.Context, from, to ChampState, keep int64) ([]int64, error) {
	return p.PromoteCAS(int32(from), int32(to), keep)
}

func (p *RedisPersistence) SaveState(ctx context.Context, state ChampState) error {
	return p.SaveMeta(int32(state))
}

func (p *RedisPersistence) LoadState(ctx context.Context) (ChampState, error) {
	meta, err := p.LoadMeta()
	if err != nil || meta == nil {
		return StateIdle, nil
	}
	return ChampState(meta.State), nil
}

func strToInt(s string) int64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
