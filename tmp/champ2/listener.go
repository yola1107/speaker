package champv2

import (
	"log"
	"time"
)

// Listener 单 Listener
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
		log.Printf("[Listener] Phase ENTER: %q at %s\n", phase, at.Format("15:04:05"))
	default:
	}
}

func (l *DefaultListener) OnLeavePhase(c *Championship, phase ChampState, at time.Time, duration time.Duration) {
	switch phase {
	case StateKO16:
		log.Printf("[Listener] Phase LEAVE: %q at %s, duration %v", phase, at.Format("15:04:05"), duration)
	default:
	}
}

func (l *DefaultListener) OnReminder(c *Championship, phase ChampState, secondsBefore int, at time.Time) {
	log.Printf("[Listener] Phase Reminder: %d seconds before %q at %s", secondsBefore, phase, at.Format("15:04:05"))
}
