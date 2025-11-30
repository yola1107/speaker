package sjnws3

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"

	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/utils/snow"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// pickValues line指定切片  num元素个数
func (s *betOrderService) pickValues(line []int, num int) ([]int, int) {
	startIdx := rand.IntN(len(line))
	return s.getNextValues(line, startIdx, num)
}

// getNextValues 从slice中获取指定数量的元素，支持循环访问
func (s *betOrderService) getNextValues(slice []int, startIdx, num int) ([]int, int) {
	length := len(slice)
	result := make([]int, num)
	for i := 0; i < num; i++ {
		index := (startIdx + i) % length
		result[i] = slice[index]
	}
	nextStartIdx := (startIdx + num) % length
	return result, nextStartIdx
}

func (s *betOrderService) getWeightedRandomValueBase() int64 {
	values := s.cfg.RollCfg.Base.UseKey
	weights := s.cfg.RollCfg.Base.Weight
	return s.getWeightedRandomValue(values, weights)
}

func (s *betOrderService) getRespinWeightedRandomValueBase() int64 {
	values := make([]int, 0)
	weights := make([]int, 0)
	switch s.scene.BonusNum {
	case _bonusNum1:
		values = s.cfg.RollCfg.Free1.UseKey
		weights = s.cfg.RollCfg.Free1.Weight
	case _bonusNum2:
		values = s.cfg.RollCfg.Free2.UseKey
		weights = s.cfg.RollCfg.Free2.Weight
	case _bonusNum3:
		values = s.cfg.RollCfg.Free3.UseKey
		weights = s.cfg.RollCfg.Free3.Weight
	}
	return s.getWeightedRandomValue(values, weights)
}

// getWeightedRandomValue 根据权重随机选取一个值
func (s *betOrderService) getWeightedRandomValue(values []int, weights []int) int64 {

	if len(values) == 0 || len(weights) == 0 {
		global.GVA_LOG.Warn("Values or weights is empty")
		return 0
	}

	if len(values) != len(weights) {
		global.GVA_LOG.Warn("Values and weights length mismatch")
		return 0
	}

	// 计算总权重
	var totalWeight = 0
	for _, w := range weights {
		if w < 0 {
			global.GVA_LOG.Warn("Negative weight found", zap.Int64("weight", int64(w)))
			return 0
		}
		totalWeight += w
	}

	if totalWeight <= 0 {
		global.GVA_LOG.Warn("Total weight is zero or negative")
		return 0
	}

	// 生成 [0, totalWeight) 区间内的随机数
	r := rand.IntN(totalWeight)

	// 累计权重并匹配随机数
	var cumulative = 0
	for i := range values {
		cumulative += weights[i]
		if r < cumulative {
			return int64(values[i])
		}
	}

	// 正常不会走到这里，防止意外
	global.GVA_LOG.Warn("No value selected in weighted random pick")
	return 0
}

// GetXYByValue 根据值计算对应的x和y坐标
// 参数: value - 要计算坐标的值
// 返回: x, y - 对应的坐标
//
//	布尔值 - 表示计算是否成功（值是否在有效范围内）
func GetXYByValue(value int) (x, y int) {

	// 根据公式反推：值 = 5*y + x
	// y是行索引，等于值除以5的整数部分
	y = value / 5
	// x是列索引，等于值除以5的余数
	x = value % 5

	return x, y
}

func checkAddSymbolLine(v int, linetoSymbol map[int][]int) bool {
	list, ok := linetoSymbol[v]
	if ok {
		//global.GVA_LOG.Debug("checkAddSymbolLine", zap.Any("list", list))
		exit := common.Containers(list, v)
		//global.GVA_LOG.Debug("数据是否已经存在了", zap.Any("exit", exit))
		if !exit {
			list = append(list, v)
			linetoSymbol[v] = list
		}
		return exit
	}
	linetoSymbol[v] = []int{v}
	//global.GVA_LOG.Debug("数据不存在返回true")
	return true
}

func getSceneKey(MemberID int64) string {
	return fmt.Sprintf("%s:%s:%d", global.GVA_CONFIG.System.Site, sceneDataKeyPrefix, MemberID)
}

/*
这段代码的功能是刷新游戏网格数据：
1. 遍历每一列（0到_colCount-1）
2. 通过加权随机算法获取一个键值Key
3. 根据Key从配置数据中获取对应的Roll数据
4. 获取当前列的符号列表SymbolList
5. 从符号列表中随机选取值填充到网格的第1、2、3行
本质上是生成一个3行N列的游戏网格，每列的符号通过加权随机方式确定。
*/
func (s *betOrderService) freshGrid() {

	if s.scene.IsRestart {

		for col := 0; col < _colCount; col++ { // 控制列
			s.reSetCol(col)
		}
		return
	}
	for col := 0; col < _colCount; col++ {
		colInfo, ok := s.ColInfoMap[col]
		if !ok {
			//防御性编码,理论上这行代码不会执行
			s.reSetCol(col)
		}
		if !colInfo.IsGetMoney {
			if s.scene.ContinueNum > 0 {
				s.setLastExitedColByContineGame(col, colInfo.NextStartIdx)
				continue
			}
			s.setLastExitedCol(col, colInfo.NextStartIdx)
			continue
		}
		s.setColGrid(col, colInfo)
	}
}

func (s *betOrderService) reSetCol(col int) {
	randList := s.getSymbolList(col)
	list, nextStartIdx := s.pickValues(randList, _rowCount)
	s.setGridCol(rowList, list, col)
	s.ColInfoMap[col] = &ColInfo{
		Col:          col,
		NextStartIdx: nextStartIdx,
		SymbolList:   list,
	}
}

func (s *betOrderService) freshRespinGrid() {
	if s.scene.IsRestart {
		for col := 0; col < _colCount; col++ { // 控制列
			randList := s.getRspinSymbolList(col)
			list, NextStartIdx := s.pickValues(randList, _rowCount)
			s.setGridCol(rowList, list, col)
			s.ColInfoMap[col] = &ColInfo{
				Col:          col,
				NextStartIdx: NextStartIdx,
				SymbolList:   list,
			}
		}
		return
	}
	for col := 0; col < _colCount; col++ {
		colInfo, ok := s.ColInfoMap[col]
		if !ok {
			//防御性编码,理论上这行代码不会执行
			s.reSetCol(col)
		}
		if !colInfo.IsGetMoney {
			if s.scene.ContinueNum > 0 {
				s.setLastExitedColByContineGame(col, colInfo.NextStartIdx)
				continue
			}
			s.setLastExitxedColByFree(col, colInfo.NextStartIdx)
			continue
		}
		s.setColGridByFree(col, colInfo)
	}

}
func (s *betOrderService) setColGridByFree(col int, colInfo *ColInfo) {
	randList := s.getRspinSymbolList(col)

	list, NextStartIdx := s.getNextValues(randList, colInfo.NextStartIdx, colInfo.NextGetSymbolNum)
	s.getColList(col, list)
	s.setGridCol(rowList, s.colList[col], colInfo.Col)
	colInfo.NextStartIdx = NextStartIdx
	colInfo.SymbolList = s.colList[col]
}

func (s *betOrderService) setLastExitxedColByFree(col, LastIdx int) {
	randList := s.getRspinSymbolList(col)
	startIdx := rand.IntN(len(randList))
	list, NextStartIdx := s.getNextValues(randList, startIdx, _rowCount)
	s.setGridCol(rowList, list, col)
	s.setNewList = make([]int, 0)
	s.reSetNewList(col)
	s.colList[col] = slices.Clone(s.setNewList)
	s.setNewList = make([]int, 0)
	s.ColInfoMap[col] = &ColInfo{
		Col:          col,
		NextStartIdx: NextStartIdx,
		SymbolList:   s.colList[col],
	}
}

func (s *betOrderService) setColGrid(col int, colInfo *ColInfo) {
	randList := s.getSymbolList(col)

	list, NextStartIdx := s.getNextValues(randList, colInfo.NextStartIdx, colInfo.NextGetSymbolNum)
	s.getColList(col, list)
	s.setGridCol(rowList, s.colList[col], colInfo.Col)
	colInfo.NextStartIdx = NextStartIdx
	colInfo.SymbolList = s.colList[col]
}
func (s *betOrderService) setLastExitedCol(col, LastIdx int) {
	randList := s.getSymbolList(col)
	startIdx := rand.IntN(len(randList))
	list, NextStartIdx := s.getNextValues(randList, startIdx, _rowCount)
	s.setGridCol(rowList, list, col)
	s.setNewList = s.setNewList[:0]
	s.reSetNewList(col)
	s.colList[col] = slices.Clone(s.setNewList)
	s.setNewList = s.setNewList[:0]
	s.ColInfoMap[col] = &ColInfo{
		Col:          col,
		NextStartIdx: NextStartIdx,
		SymbolList:   s.colList[col],
	}
}
func (s *betOrderService) setLastExitedColByContineGame(col, NextStartIdx int) {
	s.setNewList = s.setNewList[:0]
	s.reSetNewList(col)
	s.colList[col] = slices.Clone(s.setNewList)
	s.ColInfoMap[col] = &ColInfo{
		Col:          col,
		NextStartIdx: NextStartIdx,
		SymbolList:   s.colList[col],
	}
}

func (s *betOrderService) getSymbolList(col int) []int {
	Key := s.getWeightedRandomValueBase()
	if Key < 0 || int(Key) >= len(s.cfg.RollCfg.RealData) {
		global.GVA_LOG.Error("Invalid Key for RealData", zap.Int64("key", Key))
		return []int{}
	}
	Roll := s.cfg.RollCfg.RealData[Key]
	if col < 0 || col >= len(Roll) {
		global.GVA_LOG.Error("Invalid col index", zap.Int("col", col))
		return []int{}
	}
	return Roll[col]
}

func (s *betOrderService) getRspinSymbolList(col int) []int {
	Key := s.getRespinWeightedRandomValueBase()
	if Key < 0 || int(Key) >= len(s.cfg.RollCfg.RealData) {
		//防御性编码,理论上这行代码不会执行
		global.GVA_LOG.Error("Invalid Key for RealData in Respin", zap.Int64("key", Key))
		return []int{}
	}
	Roll := s.cfg.RollCfg.RealData[Key]
	if col < 0 || col >= len(Roll) {
		//防御性编码,理论上这行代码不会执行
		global.GVA_LOG.Error("Invalid col index in Respin", zap.Int("col", col))
		return []int{}
	}
	return Roll[col]
}
func (s *betOrderService) getColList(col int, list []int) {
	s.cutList = s.cutList[:0]
	s.setNewList = s.setNewList[:0]
	s.EndSetList = s.EndSetList[:0]
	s.reSetNewList(col)
	s.cutList = append(slices.Clone(s.setNewList), list...)
	common.MoveNonZeroToFrontInPlace(s.cutList)
	s.colList[col] = s.cutList[0:_rowCount]
}

func (s *betOrderService) reSetNewList(x int) {
	keyStringList := mapColList[x]
	for _, posString := range keyStringList {
		var pos *Pos
		pos, err := StringToPos(posString)
		if err != nil {
			//实际上这行代码并不会运行,为了保持代码的规范才这么写
			global.GVA_LOG.Error("StringToPos err", zap.Error(err))
			continue
		}
		s.setNewList = append(s.setNewList, s.grid[pos.Y][pos.X])
	}
}
func (s *betOrderService) setGridCol(rowList, valueList []int, col int) {
	for i, row := range rowList {
		s.setGrid(row, col, valueList[i])
	}
}

func (s *betOrderService) setGrid(row, col, value int) {
	s.grid[row][col] = value
}

func (s *betOrderService) checkScatter() {
	for y := 0; y < hisCount; y++ {
		for x := 0; x < _colCount; x++ {
			if s.grid[y][x] == _scatter {
				s.ScatterNum++
			}
		}
	}
	if s.ScatterNum >= 3 && s.scene.ContinueNum == 0 {
		s.scene.ScatterNum = s.ScatterNum
		s.scene.IsRespin = true
		s.BonusState = _freeGame
		s.scene.BonusState = _freeGame
	}
}

func (s *betOrderService) checkFreeScatter() {
	for y := 0; y < hisCount; y++ {
		for x := 0; x < _colCount; x++ {
			if s.grid[y][x] == _scatter {
				s.ScatterNum++
			}
		}
	}
	if s.ScatterNum >= 3 && s.scene.ContinueNum == 0 {
		value, ok := s.bonusMap[s.scene.BonusNum]
		if !ok {
			//防御性编码,理论上这行代码不会执行
			global.GVA_LOG.Error("Invalid BonusNum", zap.Int("bonusNum", s.scene.BonusNum))
			return
		}
		s.scene.AddFreeTimes = value.Times + (s.ScatterNum-_minScatterNum)*value.AddTimes
		s.scene.FreeTimes += value.Times + (s.ScatterNum-_minScatterNum)*value.AddTimes
		s.scene.MaxFreeTimes += value.Times + (s.ScatterNum-_minScatterNum)*value.AddTimes
		s.scene.LastFreeTimes = s.scene.FreeTimes
		s.scene.LastMaxFreeTimes = s.scene.MaxFreeTimes
	}
}

//func (s *betOrderService) DebugGrid(grid [4][5]int) []string {
//	//global.GVA_LOG.Debug("DebugGrid")
//	var strList []string
//	str := ""
//	for Y := 3; Y >= 0; Y-- {
//		str += fmt.Sprintf("\t\t\t第%d行:[", Y+1)
//		for X := 0; X < 5; X++ {
//			if X == 4 {
//				str += fmt.Sprintf("坐标(%d,%d) ", X, Y)
//				str += fmt.Sprint(grid[Y][X])
//				continue
//			}
//			str += fmt.Sprintf("坐标(%d,%d) ", X, Y)
//			str += fmt.Sprint(grid[Y][X], ",")
//		}
//		str += fmt.Sprintf("] \n")
//		strList = append(strList, str)
//	}
//	fmt.Println(str)
//	return strList
//}

// StringToPos 将字符串 "x,y" 解析为 *Pos
// 例如: "0,0" -> &Pos{0,0}
func StringToPos(s string) (*Pos, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid position format: %s, expected format: x,y", s)
	}

	x, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid x coordinate: %v", err)
	}

	y, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid y coordinate: %v", err)
	}

	return &Pos{X: x, Y: y}, nil
}

// ==================== 初始化和检查 ====================

// @checkInit ready  下注的检查准备
func (s *betOrderService) checkInit() (bool, error) {
	if err := s.initialize(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *betOrderService) initialize() error {
	s.orderSN = strconv.FormatInt(snow.GenarotorID(s.member.ID), 10)
	s.bonusAmount = decimal.NewFromFloat(0)

	//zap.L().Debug("spin", zap.Any("本次投注是否免费", s.currentSpin))
	firstRound := false
	if !s.IsFreeSpin {

		if s.scene.ContinueNum <= 0 {

			err := s.initFirstStepForSpin()
			if err != nil {
				return err
			}
			firstRound = true
		} else {
			s.initStepForNextStep()
		}
	} else {
		s.initStepForNextStep()
	}
	sn := common.GenerateOrderSn(s.member, s.lastOrder, firstRound, s.IsFreeSpin)
	s.orderSN = sn.OrderSN
	s.parentOrderSN = sn.ParentOrderSN
	s.freeOrderSN = sn.FreeOrderSN
	return nil
}

func (s *betOrderService) initFirstStepForSpin() error {
	//zap.L().Debug("进入付费初始化")
	s.client.ClientOfFreeGame.ResetFreeClean()
	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.client.SetLastMaxFreeNum(0)
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.ClientOfFreeGame.SetBetAmount(s.betAmount.InexactFloat64())
	s.client.ClientOfFreeGame.SetLastWinId(uint64(time.Now().UnixNano()))
	return nil
}

func (s *betOrderService) initStepForNextStep() {
	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple
	s.betAmount = decimal.NewFromFloat(s.client.ClientOfFreeGame.GetBetAmount())
	s.amount = decimal.Zero
	s.client.ClientOfFreeGame.ResetRoundBonus()
}

// 检查用户余额
func (s *betOrderService) checkBalance() bool {
	//if decimal.NewFromFloat(s.member.Balance).LessThan(s.betAmount) {
	//	global.GVA_LOG.Warn("checkBalance", zap.Error(errors.New("insufficient balance")))
	//	return false
	//}
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// 初始化游戏redis
func (s *betOrderService) selectGameRedis() {
	index := _gameID % int64(len(global.GVA_GAME_REDIS))
	s.gameRedis = global.GVA_GAME_REDIS[index]
}

// 获取请求上下文
func (s *betOrderService) getRequestContext() bool {
	switch {
	case !s.mdbGetMerchant():
		return false
	case !s.mdbGetMember():
		return false
	case !s.mdbGetGame():
		return false
	default:
		return true
	}
}

// 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).Mul(decimal.NewFromInt(_baseMultiplier))
	s.amount = s.betAmount
	if s.scene.ContinueNum > 0 {
		s.amount = decimal.Zero
	}
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// ==================== 常量和变量 ====================
