package champv2

import (
	"encoding/json"
	"fmt"
	"time"
)

// ==================== 阶段定义 ====================

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
	if int(s) >= 0 && int(s) < len(StateNames) {
		return StateNames[s]
	}
	return "unknown"
}

// ==================== 加载并解析配置 ====================

// 周一到周天定义： "Monday" "Tuesday" "Wednesday" "Thursday" "Friday" "Saturday" "Sunday"
var _defaultChampConfigJSON = `{
  "phases": {
    "1": {"state_name":"Registering","start":"Monday 00:00","end":"Friday 23:59","reminders":[],"keep":10000},
    "2": {"state_name":"Qualifying","start":"Saturday 18:00","end":"Saturday 21:00","reminders":[3,1],"keep":10000},
    "3": {"state_name":"KO64","start":"Sunday 08:00","end":"Sunday 09:00","reminders":[],"keep":64},
    "4": {"state_name":"KO32","start":"Sunday 09:00","end":"Sunday 10:00","reminders":[],"keep":32},
    "5": {"state_name":"KO16","start":"Sunday 10:00","end":"Sunday 11:00","reminders":[3,1],"keep":16},
    "6": {"state_name":"Final","start":"Sunday 18:00","end":"Sunday 20:00","reminders":[],"keep":8}
  }
}`

type ChampConfig struct {
	Phases map[ChampState]PhaseConfig `json:"phases"`
	Order  []ChampState               `json:"-"` // 阶段执行顺序
}

type PhaseConfig struct {
	StateName string `json:"state_name"` // 阶段名称
	StartStr  string `json:"start"`      // 配置时间
	EndStr    string `json:"end"`        // 配置时间
	Reminders []int  `json:"reminders"`  // 提前通知秒数
	Keep      int64  `json:"keep"`       // 晋级人数

	Start time.Time `json:"-"` // 解析后的开始时间
	End   time.Time `json:"-"` // 解析后的结束时间
}

// LoadChampConfig 加载并解析配置
func LoadChampConfig(jsonStr string) *ChampConfig {
	cfg, err := loadChampConfig(jsonStr)
	if err != nil {
		panic(fmt.Errorf("loadChampConfig err: %v", err))
	}
	return cfg
}

func loadChampConfig(jsonStr string) (*ChampConfig, error) {
	var cfg ChampConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, err
	}

	// 初始化 map 和 order
	if cfg.Phases == nil {
		cfg.Phases = make(map[ChampState]PhaseConfig)
	}
	if cfg.Order == nil {
		cfg.Order = []ChampState{
			StateRegistering, StateQualifying, StateKO64,
			StateKO32, StateKO16, StateFinal,
		}
	}

	monday := weekMonday(time.Now())

	// 解析每个阶段的时间
	for stateID, pc := range cfg.Phases {
		start, err := parseTime(pc.StartStr, monday)
		if err != nil {
			return nil, fmt.Errorf("parse start for %s %q: %w", pc.StateName, pc.StartStr, err)
		}
		end, err := parseTime(pc.EndStr, monday)
		if err != nil {
			return nil, fmt.Errorf("parse end for %s %q: %w", pc.StateName, pc.EndStr, err)
		}

		pc.Start = start
		pc.End = end
		cfg.Phases[stateID] = pc
	}

	return &cfg, nil
}

var weekdayMap = map[string]time.Weekday{
	"Monday":    time.Monday,
	"Tuesday":   time.Tuesday,
	"Wednesday": time.Wednesday,
	"Thursday":  time.Thursday,
	"Friday":    time.Friday,
	"Saturday":  time.Saturday,
	"Sunday":    time.Sunday,
}

// "Monday 00:00" -> time.Time
func parseTime(expr string, monday time.Time) (time.Time, error) {
	var day string
	var h, m int
	if _, err := fmt.Sscanf(expr, "%s %d:%d", &day, &h, &m); err != nil {
		return time.Time{}, fmt.Errorf("invalid time expr: %s", expr)
	}
	wd, ok := weekdayMap[day]
	if !ok {
		return time.Time{}, fmt.Errorf("invalid weekday: %s", day)
	}
	offset := int(wd - time.Monday)
	if offset < 0 {
		offset += 7
	}
	return monday.AddDate(0, 0, offset).Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute), nil
}

// GetMondayDate 获取本周周一 0点
func GetMondayDate(offsetWeek int) time.Time {
	return weekMonday(time.Now()).AddDate(0, 0, offsetWeek*7)
}

// weekMonday 获取周一时间
func weekMonday(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := t.AddDate(0, 0, -(wd - 1))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
}

// ToJSON 输出 JSON
func ToJSON(v any) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

// NextStageByOrder 获取当前阶段的后一位阶段
func (c *ChampConfig) NextStageByOrder(current ChampState) ChampState {
	for i, stage := range c.Order {
		if stage == current {
			if i+1 < len(c.Order) {
				return c.Order[i+1]
			}
			break
		}
	}
	return StateFinished // 如果是最后一个阶段，返回已完成状态
}
