package champv2

import (
	"testing"
	"time"
)

func TestChampionship_AutoAdvance(t *testing.T) {
	champ := NewChampionship(
		"champ-001",
		"Championship-Demo",
		GetScheduleTime(time.Now()),
	)
	defer champ.Stop()

	// 等待全部阶段完成
	time.Sleep(120 * time.Second)
}

//
//func TestChampionship_AutoAdvance_NonContinuous(t *testing.T) {
//	now := time.Now()
//	sched := GetScheduleTime(now)
//
//	champ := NewChampionship("c1", "TestChamp", sched)
//	defer champ.Stop()
//
//	assertState := func(expect ChampState) {
//		state := champ.CurrentState()
//		if state != expect {
//			t.Fatalf("expect %s, got %s", expect, state)
//		}
//	}
//
//	assertState(StateIdle)
//
//	time.Sleep(3 * time.Second)
//	assertState(StateRegistering)
//
//	time.Sleep(15 * time.Second)
//	assertState(StateQualifying)
//
//	time.Sleep(30 * time.Second)
//	assertState(StateKO64)
//
//	time.Sleep(20 * time.Second)
//	assertState(StateKO32)
//}
