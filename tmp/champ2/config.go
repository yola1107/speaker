package champ2

import "time"

// 阶段ID
const (
	StageIDQualifying = 1
	StageIDKnockout   = 2
	StageIDFinal      = 3
)

// 锦标赛配置
type ChampConfig struct {
	GameId int32
	RoomId int32
	Stages map[int32]*StageConfig
}

type StageConfig struct {
	StartTime int64
	EndTime   int64
	Scores    []int64
	Rounds    []int32
}

// 获取默认配置
func GetDefaultChampConfig(gameId, roomId int32) *ChampConfig {
	base := GetMondayMidnight()
	return &ChampConfig{
		GameId: gameId,
		RoomId: roomId,
		Stages: map[int32]*StageConfig{
			StageIDQualifying: {
				StartTime: base.Add(time.Hour).Unix(),
				EndTime:   base.AddDate(0, 0, 5).Add(time.Hour).Unix(),
				Scores:    []int64{3, 1, 0, 0},
				Rounds:    []int32{},
			},
			StageIDKnockout: {
				StartTime: base.AddDate(0, 0, 5).Add(time.Hour).Unix(),
				EndTime:   base.AddDate(0, 0, 5).Add(23*time.Hour + 59*time.Minute).Unix(),
				Scores:    []int64{3, 1, 0, 0},
				Rounds:    []int32{64, 32, 16, 8},
			},
			StageIDFinal: {
				StartTime: base.AddDate(0, 0, 5).Add(24 * time.Hour).Unix(),
				EndTime:   base.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute).Unix(),
				Scores:    []int64{5, 3, 1, 0},
				Rounds:    []int32{},
			},
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
