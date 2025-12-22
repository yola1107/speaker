package champv2

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis"
)

// 获取演示赛程
// GetDemoScheduleTime 返回演示赛程，所有时间相对 now
func GetDemoScheduleTime(now time.Time) *ChampConfig {
	return &ChampConfig{
		Phases: map[ChampState]PhaseConfig{
			StateRegistering: {
				StateName: "Registering",
				Start:     now.Add(5 * time.Second),
				End:       now.Add(10 * time.Second),
				Keep:      10000, // 注册阶段允许最多10000人
			},
			StateQualifying: {
				StateName: "Qualifying",
				Start:     now.Add(20 * time.Second),
				End:       now.Add(30 * time.Second),
				Reminders: []int{3, 1}, Keep: 10000, // 海选赛全员进入下一阶段
			},
			StateKO64: {
				StateName: "KO64",
				Start:     now.Add(40 * time.Second),
				End:       now.Add(50 * time.Second),
				Keep:      64, // 64强赛保留64人
			},
			StateKO32: {
				StateName: "KO32",
				Start:     now.Add(60 * time.Second),
				End:       now.Add(70 * time.Second),
				Keep:      32, // 32强赛保留32人
			},
			StateKO16: {
				StateName: "KO16",
				Start:     now.Add(80 * time.Second),
				End:       now.Add(90 * time.Second),
				Keep:      16, // 16强赛保留16人
			},
			StateFinal: {
				StateName: "Final",
				Start:     now.Add(100 * time.Second), End: now.Add(110 * time.Second),
				Keep: 8, // 决赛保留8人
			},
		},
		Order: []ChampState{
			StateRegistering, StateQualifying, StateKO64, StateKO32, StateKO16, StateFinal,
		},
	}
}

func TestChampionship_AutoAdvance(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: "192.168.152.128:6379",
		DB:   0,
	})
	defer rdb.Close()

	champ := NewChampionship(
		180,
		1,
		GetDemoScheduleTime(time.Now().Add(-1*time.Second)),
		rdb,
	)
	defer champ.Stop()

	// 等待全部阶段完成
	time.Sleep(120 * time.Second)
}

func TestLoadChampConfig(t *testing.T) {
	cfg := LoadChampConfig(_defaultChampConfigJSON)

	for _, stateID := range cfg.Order {
		phase := cfg.Phases[stateID]
		fmt.Printf("%-12s: start=%s, end=%s, reminders=%v, keep=%v\n",
			phase.StateName,
			phase.Start.Format("2006-01-02 15:04"),
			phase.End.Format("2006-01-02 15:04"),
			phase.Reminders,
			phase.Keep,
		)
	}
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
