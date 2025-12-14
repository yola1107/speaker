package champ2

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type ChampState int32

const (
	StateIdle       ChampState = iota // 初始状态（未开始）
	StateQualifying                   // 海选赛
	StateKnockout64                   // 淘汰赛64进32
	StateKnockout32                   // 淘汰赛32进16
	StateKnockout16                   // 淘汰赛16进8
	StateKnockout8                    // 淘汰赛8进4（实际是8进决赛桌）
	StateFinal                        // 决赛桌
	StateFinished                     // 锦标赛结束
)

type Persistence interface {
	Promote(ctx context.Context, from, to ChampState, keep int64) ([]int64, error)
	SaveState(ctx context.Context, state ChampState) error
	LoadState(ctx context.Context) (ChampState, error)
}

type FSM struct {
	mu        sync.Mutex
	state     ChampState
	p         Persistence
	cfg       *ChampConfig
	ctx       context.Context
	cancel    context.CancelFunc
	scheduler Scheduler
}

func NewFSM(p Persistence, cfg *ChampConfig) *FSM {
	ctx, cancel := context.WithCancel(context.Background())
	state := StateIdle
	if p != nil {
		if s, err := p.LoadState(ctx); err == nil {
			state = s
		}
	}

	return &FSM{
		state:  state,
		p:      p,
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		scheduler: NewHeapScheduler(
			WithHeapContext(ctx),
			WithHeapWorkerPool(32, 64),
		),
	}
}

func (f *FSM) Start() {
	now := time.Now().Unix()
	log.Infof("[FSM] start state=%d now=%d", f.state, now)
	f.catchUp(now)
	f.registerFuture()
}

func (f *FSM) Stop() {
	f.cancel()
	f.scheduler.Stop()
}

func (f *FSM) registerFuture() {
	f.registerAt(f.cfg.Stages[StageIDQualifying].EndTime, StateQualifying, StateKnockout64)
	f.registerAt(f.cfg.Stages[StageIDKnockout].EndTime, StateKnockout8, StateFinal)
	f.registerAt(f.cfg.Stages[StageIDFinal].EndTime, StateFinal, StateFinished)
}

func (f *FSM) registerAt(ts int64, from, to ChampState) {
	delay := time.Until(time.Unix(ts, 0))
	if delay <= 0 {
		go f.onTimeout(from, to)
		return
	}
	f.scheduler.Once(delay, func() { f.onTimeout(from, to) })
}

func (f *FSM) onTimeout(from, to ChampState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state != from {
		return
	}
	log.Infof("[FSM] timeout %d -> %d", from, to)
	if f.p != nil {
		if _, err := f.p.Promote(f.ctx, from, to, 0); err != nil {
			log.Errorf("[FSM] promote failed: %v", err)
			return
		}
		_ = f.p.SaveState(f.ctx, to)
	}
	f.state = to
}

func (f *FSM) catchUp(now int64) {
	for {
		next, ok := f.nextStateByTime(now)
		if !ok {
			break
		}
		f.onTimeout(f.state, next)
	}
}

func (f *FSM) nextStateByTime(now int64) (ChampState, bool) {
	switch f.state {
	case StateQualifying:
		if now >= f.cfg.Stages[StageIDQualifying].EndTime {
			return StateKnockout64, true
		}
	case StateKnockout64:
		return StateKnockout32, true
	case StateKnockout32:
		return StateKnockout16, true
	case StateKnockout16:
		return StateKnockout8, true
	case StateKnockout8:
		if now >= f.cfg.Stages[StageIDKnockout].EndTime {
			return StateFinal, true
		}
	case StateFinal:
		if now >= f.cfg.Stages[StageIDFinal].EndTime {
			return StateFinished, true
		}
	}
	return 0, false
}

func (f *FSM) State() ChampState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}
