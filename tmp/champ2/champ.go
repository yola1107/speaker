package champv2

import (
	"context"
	"log"
	"sync"
	"time"
)

type Championship struct {
	ID         string
	Name       string
	config     *ChampConfig
	current    ChampState
	listener   Listener
	workerPool *WorkerPool
	scheduler  Scheduler
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc

	runtime map[ChampState]time.Time
}

func NewChampionship(id, name string, config *ChampConfig) *Championship {
	ctx, cancel := context.WithCancel(context.Background())
	workerPool := NewWorkerPool(8, 64)

	c := &Championship{
		ID:         id,
		Name:       name,
		config:     config,
		current:    StateIdle,
		listener:   NewPhaseListener(),
		workerPool: workerPool,
		scheduler: NewHeapScheduler(
			WithHeapContext(ctx),
			WithWorkerPool(workerPool),
		),
		runtime: make(map[ChampState]time.Time),
		ctx:     ctx,
		cancel:  cancel,
	}
	c.scheduleAll()

	log.Printf("NewChampionship. id=%s, name=%s, current=%q", c.ID, c.Name, c.current)
	return c
}

func (c *Championship) Stop() {
	c.scheduler.Stop()
	c.workerPool.Stop()
	c.cancel()
}

func (c *Championship) CurrentState() ChampState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// 调度所有阶段
func (c *Championship) scheduleAll() {
	phases := []struct {
		state ChampState
		cfg   PhaseConfig
	}{
		{StateRegistering, c.config.Register},
		{StateQualifying, c.config.Qualify},
		{StateKO64, c.config.KO64},
		{StateKO32, c.config.KO32},
		{StateKO16, c.config.KO16},
		{StateFinal, c.config.Final},
	}

	for _, p := range phases {
		// enter
		c.scheduler.ScheduleAt(p.cfg.Start, func(s ChampState, t time.Time) func() {
			return func() { c.enterPhase(s, t) }
		}(p.state, p.cfg.Start))

		// leave
		c.scheduler.ScheduleAt(p.cfg.End, func(s ChampState, t time.Time) func() {
			return func() { c.leavePhase(s, t) }
		}(p.state, p.cfg.End))

		// reminders
		for _, sec := range p.cfg.ReminderSeconds {
			notifyAt := p.cfg.Start.Add(-time.Duration(sec) * time.Second)
			if notifyAt.Before(time.Now()) {
				continue
			}
			c.scheduler.ScheduleAt(notifyAt, func(s ChampState, sec int, t time.Time) func() {
				return func() { c.listener.OnReminder(c, s, sec, t) }
			}(p.state, sec, notifyAt))
		}
	}
}

func (c *Championship) enterPhase(s ChampState, at time.Time) {
	c.mu.Lock()
	c.current = s
	c.runtime[s] = at
	c.mu.Unlock()
	log.Printf("[%s] >>> enter %s\n", c.Name, s)
	c.listener.OnEnterPhase(c, s, at)
}

func (c *Championship) leavePhase(s ChampState, at time.Time) {
	c.mu.RLock()
	start, ok := c.runtime[s]
	c.mu.RUnlock()
	duration := time.Duration(0)
	if ok {
		duration = at.Sub(start)
	}
	c.listener.OnLeavePhase(c, s, at, duration)
	log.Printf("[%s] <<< leave %s (%v)\n\n", c.Name, s, duration)
}
