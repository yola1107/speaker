package sjnws3

type ColInfo struct {
	Col              int
	NextStartIdx     int
	NextGetSymbolNum int
	IsGetMoney       bool
	SymbolList       []int
}

// 中奖结果
type winResult struct {
	Symbol             int                       `json:"symbol"`             //  符号
	SymbolCount        int                       `json:"symbolCount"`        //  数量
	LineCount          int                       `json:"lineCount"`          //  中奖线数量
	BaseLineMultiplier int                       `json:"baseLineMultiplier"` //  中奖线倍数
	TotalMultiplier    int                       `json:"totalMultiplier"`    //  总倍数
	WinPositions       [_rowCount][_colCount]int `json:"winPositions"`
}

type int64GridY = [_rowCount][_colCount]int
type HisGridY = [hisCount][_colCount]int
type Pos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type BaseSpinResult struct {
	StepMultiplier  int64                      `json:"step_multiplier"`
	AddFreeTime     int                        `json:"add_free_time"`
	WinDetails      [_rowCount * _colCount]int `json:"win_grid"`
	CardDetails     [_rowCount * _colCount]int `json:"card_details"`
	BonusDetails    [hisCount * _colCount]int  `json:"bonus_details"`
	Cards           int64GridY                 `json:"cards"`
	WinCards        HisGridY                   `json:"win_cards"`
	CurrentWin      float64                    `json:"current_win"`
	IsRespin        bool                       `json:"is_respin"`
	CurrentRespin   bool                       `json:"current_respin"`
	Osn             string                     `json:"osn"`
	Balance         float64                    `json:"balance"`
	BetAmount       float64                    `json:"bet_amount"`
	FreeTotalAmount float64                    `json:"free_total_amount"`
}
