package sjnws3

import (
	"fmt"
	"sort"

	"egame-grpc/game/common"
	"egame-grpc/global"

	"github.com/shopspring/decimal"
)

// ==================== 基础旋转逻辑 ====================

func (s *betOrderService) baseSpin() (*BaseSpinResult, error) {
	if s.scene.ContinueNum <= 0 {
		s.scene.LastTotalWin = 0
		s.scene.TotalFreeWin = 0
		s.scene.TotalWin = 0
	}
	s.freshGrid() //刷新符号
	s.scene.LastGrid = s.grid

	s.checkReward()  //检测奖励
	s.checkScatter() //检查夺宝符号
	s.drop()         //检查符号掉落
	re := s.setBaseResult()
	s.updateHistoryFreeDate()
	return re, nil
}

func (s *betOrderService) reSpinBase() (*BaseSpinResult, error) {
	if s.scene.ContinueNum <= 0 {
		s.scene.FreeTimes-- //免费次数减1
		s.scene.LastFreeTimes--
		s.client.ClientOfFreeGame.IncrFreeTimes()
	}
	s.freshRespinGrid() //刷新符号
	s.scene.LastGrid = s.grid
	s.checkFreeReward()         //检测奖励
	s.checkFreeScatter()        //检查夺宝符号
	s.freeDrop()                //检查符号掉落
	result := s.setBaseResult() //设置结果
	s.updateHistoryFreeDate()
	return result, nil
}

// ==================== 中奖信息处理 ====================

func (s *betOrderService) checkReward() {
	s.midgrid = s.grid
	ResultList := make([]*winResult, 0)
	linetoSymbol := make(map[int][]int)
	for i, line := range s.bonusLine {
		ResultList = s.checkLineEx(i, line, ResultList, linetoSymbol)
	}
	if len(ResultList) == 0 {
		s.normalGameStopContinue()
		return
	}
	mul := 0
	for _, result := range ResultList {
		mul += result.BaseLineMultiplier
	}

	s.winResult = ResultList
	s.StepMulTy = int64(mul)
	s.scene.ContinueNum++
	s.setContinueMulti()
	s.extraMul()
}

func (s *betOrderService) checkFreeReward() {
	s.midgrid = s.grid
	ResultList := make([]*winResult, 0)
	linetoSymbol := make(map[int][]int)
	for i, line := range s.bonusLine {
		ResultList = s.checkLineEx(i, line, ResultList, linetoSymbol)
	}
	if len(ResultList) == 0 {
		s.freeGameStopContinue()
		return
	}
	mul := 0
	for _, result := range ResultList {
		mul += result.BaseLineMultiplier
	}
	s.winResult = ResultList
	s.StepMulTy = int64(mul)
	//global.GVA_LOG.Debug("中奖信息:", zap.Any("倍数:", s.winGrid))
	//global.GVA_LOG.Debug("中奖信息:", zap.Any("倍数:", s.StepMulTy))
	s.scene.ContinueNum++
	s.setContinueMulti()
	s.extraMul()
}

func (s *betOrderService) extraMul() {
	if s.scene.ContinueNum == 0 {
		//zap.L().Debug("没有中奖")
		//没有中奖不会继续计算
		return
	}
	if !s.IsFreeSpin {
		//非免费模式下连续中奖
		Max := len(s.cfg.BaseStreakMulti)
		if s.scene.ContinueNum >= Max {
			s.ExMul = int64(s.cfg.BaseStreakMulti[Max-1])

			return
		}
		s.ExMul = int64(s.cfg.BaseStreakMulti[s.scene.ContinueNum-1])

		return
	}
	if !common.Containers([]int{1, 2, 3}, s.scene.BonusNum) {
		global.GVA_LOG.Error("缓存中出现了错误的数据")
		return
	}
	//免费模式下连续中奖
	mulTi := s.bonusMap[s.scene.BonusNum].Multi
	Max := len(mulTi)
	if s.scene.ContinueNum >= Max {
		s.ExMul = int64(mulTi[Max-1])

		return
	}
	s.ExMul = int64(mulTi[s.scene.ContinueNum-1])
}

func (s *betOrderService) setContinueMulti() {
	if s.scene.ContinueNum == 0 {
		//zap.L().Debug("没有中奖")
		//没有中奖不会继续计算
		s.PreMul = 1
		s.scene.ContinueMulti = 1
		return
	}
	if !s.IsFreeSpin {
		//非免费模式下连续中奖
		Max := len(s.cfg.BaseStreakMulti)
		if s.scene.ContinueNum >= Max {
			s.scene.ContinueMulti = s.cfg.BaseStreakMulti[Max-1]
			return
		}
		s.scene.ContinueMulti = s.cfg.BaseStreakMulti[s.scene.ContinueNum]
		return
	}
	//if !common.Containers([]int{1, 2, 3}, s.scene.BonusNum) {
	//	global.GVA_LOG.Error("缓存中出现了错误的数据")
	//	return
	//}
	//免费模式下连续中奖
	mulTi := s.bonusMap[s.scene.BonusNum].Multi
	Max := len(mulTi)
	if s.scene.ContinueNum >= Max {

		s.scene.ContinueMulti = mulTi[Max-1]
		return
	}
	s.scene.ContinueMulti = mulTi[Max-1]
}

func (s *betOrderService) setPretraMulForNormal() {
	if s.IsFreeSpin {
		mulTi, ok := s.bonusMap[s.scene.BonusNum]
		if ok && mulTi != nil {
			s.PreMul = int64(mulTi.Multi[s.scene.ContinueNum])
			s.scene.ContinueMulti = int(s.PreMul)
			return
		}
		//这里理论是不会执行
		s.PreMul = 6
		s.scene.ContinueMulti = int(s.PreMul)
		return
	}
	s.PreMul = 1
	s.scene.ContinueMulti = int(s.PreMul)
}

func (s *betOrderService) setPretraMul() {
	if s.scene.ContinueNum == 0 {
		//没有中奖不会继续计算
		s.setPretraMulForNormal()
		return
	}
	if !s.IsFreeSpin {
		//非免费模式下连续中奖
		Max := len(s.cfg.BaseStreakMulti)
		if s.scene.ContinueNum >= Max {
			s.PreMul = int64(s.cfg.BaseStreakMulti[Max-1])
			s.scene.ContinueMulti = int(s.PreMul)
			return
		}
		s.PreMul = int64(s.cfg.BaseStreakMulti[s.scene.ContinueNum])
		s.scene.ContinueMulti = int(s.PreMul)
		return
	}
	//if !common.Containers([]int{1, 2, 3}, s.scene.BonusNum) {
	//	global.GVA_LOG.Error("缓存中出现了错误的数据")
	//	s.scene.BonusNum = 3 //强制修正错误数据
	//	s.PreMul = 6
	//	s.scene.ContinueMulti = 6
	//	return
	//}
	//免费模式下连续中奖
	mulTi := s.bonusMap[s.scene.BonusNum].Multi
	Max := len(mulTi)
	if s.scene.ContinueNum >= Max {
		s.PreMul = int64(mulTi[Max-1])
		s.scene.ContinueMulti = int(s.PreMul)
		return
	}
	s.PreMul = int64(mulTi[s.scene.ContinueNum])
	s.scene.ContinueMulti = int(s.PreMul)
}

func (s *betOrderService) checkLineEx(i int, Line []*Pos, ResultList []*winResult, linetoSymbol map[int][]int) []*winResult {
	//global.GVA_LOG.Debug("CheckLineEx:", zap.Any("i", i))
	symbolListx := make([]int, 0, 5)
	for _, pos := range Line {
		symbolListx = append(symbolListx, s.grid[pos.Y][pos.X])
	}
	RetList := checkConsecutiveWithWildcard(symbolListx)
	if len(RetList) == 0 || RetList == nil {
		return ResultList
	}
	//global.GVA_LOG.Debug("RetList:", zap.Any("RetList", RetList))
	for _, result := range RetList {
		_, ok := s.symbolMulMap[result.Value][result.Count]
		if checkAddSymbolLine(result.Value, linetoSymbol) && ok {
			win := &winResult{}
			win.Symbol = result.Value
			win.SymbolCount = result.Count
			win.LineCount = i
			win.BaseLineMultiplier = s.symbolMulMap[result.Value][result.Count]
			s.winDetails[i] = 1
			s.checkWinPos(result.Value, result.StartIndex, result.Count, Line)
			ResultList = append(ResultList, win)
		}
	}
	//global.GVA_LOG.Debug("ResultList:", zap.Any("ResultList", ResultList))
	return ResultList
}

func (s *betOrderService) checkWinPos(Symbol, start, count int, Line []*Pos) {
	//zap.L().Debug("checkWinPos:", zap.Any("Symbol", Symbol),
	//	zap.Any("count", count),
	//	zap.Any("Line", Line))
	if count >= _minCount && start == 0 {
		for i := 0; i < count; i++ {
			s.winGrid[Line[i].Y][Line[i].X] = 1
			s.midgrid[Line[i].Y][Line[i].X] = 0
			key := PositionMap[fmt.Sprintf("%d,%d", Line[i].X, Line[i].Y)]
			s.ColInfoMap[Line[i].X].SymbolList[key] = 0
			s.setWinCol(Line[i].X)
			if s.grid[Line[i].Y][Line[i].X] == _wild {
				s.winCards[Line[i].Y][Line[i].X] = _wild
				continue
			}
			s.winCards[Line[i].Y][Line[i].X] = Symbol
		}
	}
}

// ==================== 符号掉落逻辑 ====================

type SceneCol struct {
	Col           int  `json:"col"`
	NextStartIdx  int  `json:"next_start_idx"`
	NextSymbolNum int  `json:"next_symbol_num"`
	IsGetMoney    bool `json:"is_get_money"`
}

func (s *betOrderService) loadSceneColList() {
	for _, col := range s.scene.SceneColList {
		s.ColInfoMap[col.Col] = &ColInfo{
			Col:              col.Col,
			NextStartIdx:     col.NextStartIdx,
			NextGetSymbolNum: col.NextSymbolNum,
			IsGetMoney:       col.IsGetMoney,
		}
	}
}

// Drop 处理下落逻辑
func (s *betOrderService) drop() {
	s.SetnextSceneCol()
	s.nextGrid = s.midgrid
	s.scene.NextGrid = s.midgrid
	if len(s.winCol) == 0 {
		//global.GVA_LOG.Debug("没有中奖")
		return
	}
	//global.GVA_LOG.Debug("有中奖")
	s.continueNormalGame()
	//zap.L().Debug("下落消除后符号")
	//s.DebugGrid(s.nextGrid)
}

func (s *betOrderService) freeDrop() {
	s.SetnextSceneCol()
	s.nextGrid = s.midgrid
	s.scene.NextGrid = s.midgrid
	if len(s.winCol) == 0 {
		//global.GVA_LOG.Debug("没有中奖")
		return
	}
	s.continueFreeGame()
	//zap.L().Debug("下落消除后符号")
	//s.DebugGrid(s.nextGrid)
}

// setWinPos  检查到某个位置上的符号是获得奖励时，标记成0
func (s *betOrderService) setWinCol(x int) {
	if common.Containers(s.winCol, x) {
		return
	}
	s.winCol = append(s.winCol, x)
}

func (s *betOrderService) SetnextSceneCol() {
	//zap.L().Debug("执行消除,构建下一轮的数据")
	nextSceneCol := []*SceneCol{}
	for x := 0; x < _colCount; x++ {
		newSceneCol := &SceneCol{}
		if common.Containers(s.winCol, x) {
			NextGetNum, _ := common.MoveNonZeroToFrontInPlace(s.ColInfoMap[x].SymbolList)
			newSceneCol = &SceneCol{
				Col:           x,
				NextStartIdx:  s.ColInfoMap[x].NextStartIdx,
				NextSymbolNum: NextGetNum,
				IsGetMoney:    true,
			}
		} else {
			newSceneCol = &SceneCol{
				Col:           x,
				NextStartIdx:  s.ColInfoMap[x].NextStartIdx,
				NextSymbolNum: _rowCount,
				IsGetMoney:    false,
			}
		}
		for i := 0; i < _rowCount; i++ {
			s.midgrid[i][x] = s.ColInfoMap[x].SymbolList[i]
		}
		nextSceneCol = append(nextSceneCol, newSceneCol)
	}
	s.nextGrid = s.midgrid
	s.scene.SceneColList = nextSceneCol
}

// ==================== 结果处理 ====================

func (s *betOrderService) setBaseResult() *BaseSpinResult {
	result := &BaseSpinResult{
		Cards:      s.grid,
		WinDetails: [_rowCount * _colCount]int{},
	}
	//zap.L().Debug("结算:", zap.Any("s.winCards", s.winCards))
	s.bonusAmount = s.betAmount.Mul(decimal.NewFromInt(s.StepMulTy * s.ExMul)).Div(decimal.NewFromInt(_baseMultiplier))
	SpinAmount := s.bonusAmount.Round(2).InexactFloat64()
	result.CurrentWin = SpinAmount
	//global.GVA_LOG.Debug("是不是免费:", zap.Any("free", s.currentSpin))
	result.CurrentRespin = s.IsFreeSpin
	result.Cards = s.grid
	result.WinCards = s.winCards
	result.WinDetails = s.winDetails
	result.StepMultiplier = s.StepMulTy + s.ExMul
	result.Osn = s.orderSN
	s.scene.Win = SpinAmount
	if s.IsFreeSpin {
		s.scene.FreeAmount = decimal.NewFromFloat(s.scene.FreeAmount).
			Add(decimal.NewFromFloat(SpinAmount)).Round(2).InexactFloat64()
		s.scene.freeTotalMoney += SpinAmount
		result.FreeTotalAmount = s.scene.FreeAmount
	}
	return s.setCardsDetail(result)
}

func (s *betOrderService) setCardsDetail(result *BaseSpinResult) *BaseSpinResult {
	for y := 0; y < _rowCount; y++ {
		for x := 0; x < _colCount; x++ {
			result.CardDetails[y*_colCount+x] = s.grid[y][x]
		}
	}
	for y := 0; y < hisCount; y++ {
		for x := 0; x < _colCount; x++ {
			if s.winCards[y][x] > 0 {
				result.BonusDetails[y*_colCount+x] = 1
			}
		}
	}
	return result
}

// ==================== 中奖检测辅助函数 ====================

type Result struct {
	Value      int // 连续的符号值
	Count      int // 连续的个数
	StartIndex int // 连续序列的起始索引
}

// checkConsecutiveWithWildcard 检查5元素切片中连续3个及以上可视为相同的元素
// 11作为通配符可代表任何元素，返回所有符合条件的结果（取最长连续数及对应的起始索引）
// 新增限制：只有包含索引0的序列才算有效
func checkConsecutiveWithWildcard(slice []int) []Result {
	type tempResult struct {
		count      int
		startIndex int
	}
	resultMap := make(map[int]tempResult) // 存储每个符号的最大连续数和起始索引
	if len(slice) != 5 {
		return nil // 确保输入是5元素切片
	}

	// 所有包含索引0的连续子序列范围（按长度从长到短）
	// 只保留包含索引0的序列
	ranges := [][2]int{
		{0, 4}, // 长度5: 索引0-4（包含0）
		{0, 3}, // 长度4: 索引0-3（包含0）
		{0, 2}, // 长度3: 索引0-2（包含0）
	}

	// 遍历所有包含索引0的连续子序列
	for _, r := range ranges {
		start, end := r[0], r[1]
		length := end - start + 1
		if length < 3 {
			continue // 只处理3个及以上的序列
		}

		// 提取子序列并检查是否可视为相同元素
		subSlice := slice[start : end+1]
		symbol, valid := getValidSymbol(subSlice)
		if valid {
			// 只保留最长的连续数，如果长度相同则保留较早出现的（这里start都是0，所以不会有此情况）
			if current, exists := resultMap[symbol]; !exists || length > current.count {
				resultMap[symbol] = tempResult{
					count:      length,
					startIndex: start,
				}
			}
		}
	}

	// 转换map为结果切片并按个数排序
	results := make([]Result, 0, len(resultMap))
	for val, temp := range resultMap {
		results = append(results, Result{
			Value:      val,
			Count:      temp.count,
			StartIndex: temp.startIndex,
		})
	}

	// 按连续个数从大到小排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Count > results[j].Count
	})

	return results
}

// 检查子序列是否可通过通配符转换为全相同元素，返回符号值和是否有效
func getValidSymbol(sub []int) (int, bool) {
	nonWildcards := make([]int, 0)
	for _, v := range sub {
		if v != _wild {
			nonWildcards = append(nonWildcards, v)
		}
	}

	// 全是通配符的情况（视为wild）
	if len(nonWildcards) == 0 {
		return _wild, true
	}

	// 非通配符元素必须全部相同
	base := nonWildcards[0]
	for _, v := range nonWildcards[1:] {
		if v != base {
			return 0, false // 存在不同的非通配符元素，无效
		}
	}

	return base, true // 有效，符号为非通配符的基准值
}
