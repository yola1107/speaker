// game/bxkh2/pb/bxkh2.pb.go
package pb

// 下注响应
type Bxkh2_BetOrderResponse struct {
	OrderSn       string         `json:"order_sn"`
	Balance       float64        `json:"balance"`
	BetAmount     float64        `json:"bet_amount"`
	CurrentWin    float64        `json:"current_win"`
	RoundWin      float64        `json:"round_win"`
	TotalWin      float64        `json:"total_win"`
	FreeTotalWin  float64        `json:"free_total_win"`
	IsFree        bool           `json:"is_free"`
	Review        int64          `json:"review"`
	WinInfo       *Bxkh2_WinInfo `json:"win_info"`
	Cards         []int64        `json:"cards"`
	WinGrid       []int64        `json:"win_grid"`
	FreeMultiple  int64          `json:"free_multiple"`
	OriginalCards []int64        `json:"original_cards"`
}

// 中奖信息
type Bxkh2_WinInfo struct {
	Next          bool            `json:"next"`
	Over          bool            `json:"over"`
	Multi         int64           `json:"multi"`
	State         int8            `json:"state"`
	FreeNum       uint64          `json:"free_num"`
	FreeTime      uint64          `json:"free_time"`
	TotalFreeTime uint64          `json:"total_free_time"`
	IsRoundOver   bool            `json:"is_round_over"`
	AddFreeTime   int64           `json:"add_free_time"`
	ScatterCount  int64           `json:"scatter_count"`
	WinArr        []*Bxkh2_WinArr `json:"win_arr"`
}

// 单个中奖信息
type Bxkh2_WinArr struct {
	Val     int64 `json:"val"`
	RoadNum int64 `json:"road_num"`
	StarNum int64 `json:"star_num"`
	Odds    int64 `json:"odds"`
	Mul     int64 `json:"mul"`
}
