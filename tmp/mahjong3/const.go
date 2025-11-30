package mahjong

const _gameID = 18943
const GameID = _gameID

const _baseMultiplier = 20

const (
	_rowCount    = 5 // 行数
	_colCount    = 5 // 列数
	_rowCountWin = 4 // 奖励行数
)

const (
	_blank    int64 = 0
	ERTIAO    int64 = 1  //二条
	ERTONG    int64 = 2  //二筒
	WUTIAO    int64 = 3  //五条
	WUTONG    int64 = 4  //五筒
	BAWAN     int64 = 5  //八万
	BAIBAN    int64 = 6  //白板
	ZHONG     int64 = 7  //红中
	FA        int64 = 8  //发财
	_treasure int64 = 9  //胡scatter
	_wild     int64 = 10 //百搭wild

	//金符号
	ERTIAO_k int64 = 11 //二条
	ERTONG_k int64 = 12 //二筒
	WUTIAO_k int64 = 13 //五条
	WUTONG_k int64 = 14 //五筒
	BAWAN_k  int64 = 15 //八万
	BAIBAN_k int64 = 16 //白板
	ZHONG_k  int64 = 17 //红中
	FA_k     int64 = 18 //发财
)

const _minMatchCount = 3 // 最小中奖数量
const _goldSymbol = 10   // 金符号

var _baseSpecific = [5]int{-1, -1, -1, -1, -1}

const (
	_spinTypeBase    = 1  //普通
	_spinTypeBaseEli = 11 //普通消除
	_spinTypeFree    = 21 //免费
	_spinTypeFreeEli = 22 //免费消除
)

const runStateNormal = 0   //普通
const runStateFreeGame = 1 //免费
