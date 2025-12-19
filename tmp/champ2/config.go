package champv2

import "time"

// ChampState 阶段状态
type ChampState int32

const (
	StateIdle ChampState = iota
	StateRegistering
	StateQualifying
	StateKO64
	StateKO32
	StateKO16
	StateFinal
	StateFinished
)

var StateNames = []string{"idle", "registering", "qualifying", "ko64", "ko32", "ko16", "final", "finished"}

func (s ChampState) String() string {
	if s >= 0 && s < ChampState(len(StateNames)) {
		return StateNames[s]
	}
	return "Unknown"
}

// ChampConfig 阶段配置
type ChampConfig struct {
	Register PhaseConfig
	Qualify  PhaseConfig
	KO64     PhaseConfig
	KO32     PhaseConfig
	KO16     PhaseConfig
	Final    PhaseConfig
}

type PhaseConfig struct {
	Start           time.Time
	End             time.Time
	ReminderSeconds []int
	//Run             PhaseRuntime
}

type PhaseRuntime struct {
	Start    time.Time
	End      time.Time
	Duration time.Duration
}

// 获取演示赛程
func GetScheduleTime(now time.Time) *ChampConfig {
	return &ChampConfig{
		Register: PhaseConfig{
			Start: now.Add(5 * time.Second),
			End:   now.Add(10 * time.Second),
		},
		Qualify: PhaseConfig{
			Start:           now.Add(20 * time.Second),
			End:             now.Add(30 * time.Second),
			ReminderSeconds: []int{3, 1},
		},
		KO64: PhaseConfig{
			Start: now.Add(40 * time.Second),
			End:   now.Add(50 * time.Second),
		},
		KO32: PhaseConfig{
			Start: now.Add(60 * time.Second),
			End:   now.Add(70 * time.Second),
		},
		KO16: PhaseConfig{
			Start: now.Add(80 * time.Second),
			End:   now.Add(90 * time.Second),
		},
		Final: PhaseConfig{
			Start: now.Add(100 * time.Second),
			End:   now.Add(110 * time.Second),
		},
	}
}

func GetMondayMidnight() time.Time {
	now := time.Now()
	offset := int(time.Monday - now.Weekday())
	if offset > 0 {
		offset -= 7
	}
	return now.AddDate(0, 0, offset).Truncate(24 * time.Hour)
}
