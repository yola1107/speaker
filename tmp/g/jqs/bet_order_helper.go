package jqs

import (
	"fmt"

	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// 获取请求上下文
func (s *betOrderService) getRequestContext() error {
	mer, mem, ga, err := common.GetRequestContext(s.req)
	if err != nil {
		global.GVA_LOG.Error("getRequestContext error.", zap.Error(err))
		return err
	}
	s.merchant, s.member, s.game = mer, mem, ga
	return nil
}

// 更新下注金额
func (s *betOrderService) updateBetAmount() bool {
	s.betAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(_baseMultiplier))
	if s.betAmount.LessThanOrEqual(decimal.Zero) {
		global.GVA_LOG.Warn("updateBetAmount",
			zap.Error(fmt.Errorf("invalid request params: [%v,%v]", s.req.BaseMoney, s.req.Multiple)))
		return false
	}
	return true
}

// 检查用户余额
func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

// 更新奖金金额
func (s *betOrderService) updateBonusAmount() {
	// RTP测试模式或无倍数时直接返回
	if s.debug.open || s.stepMultiplier == 0 {
		s.bonusAmount = decimal.Zero
		return
	}
	s.bonusAmount = decimal.NewFromFloat(s.req.BaseMoney).
		Mul(decimal.NewFromInt(s.req.Multiple)).
		Mul(decimal.NewFromInt(s.stepMultiplier))
	if s.bonusAmount.GreaterThan(decimal.Zero) {
		rounded := s.bonusAmount.Round(2).InexactFloat64()
		s.scene.TotalWin += rounded
		s.scene.RoundWin += rounded
		if s.isFreeRound {
			s.scene.FreeWin += rounded
		}
	}
}

// 初始化step预设数据 - 从滚轴生成
func (s *betOrderService) initSymbolGrid() {
	switch s.scene.Stage {
	case _spinTypeBase:
		// 判定是否触发Re-spin
		if isHit(s.gameConfig.RespinTriggerRate) {
			s.scene.Stage = _spinTypeFree
			s.isFreeRound = true
			s.state = 1 // 进入免费模式
			// Re-spin模式不需要设置次数，会一直spin直到中奖
		} else if isHit(s.gameConfig.FakeRespinTriggerRate) {
			s.state = 3 // 炸胡状态
		}
	case _spinTypeFree:
		// Re-spin：中间列固定百搭；未中奖则 NextStage 仍在 baseSpin 中设为 Free
	}
	s.symbolGrid = s.getSceneSymbol()
}

func (s *betOrderService) findWinInfo() {
	var winInfos []WinInfo
	var totalWinGrid int64Grid

	for i, line := range s.gameConfig.Lines {
		allWild := true
		for _, p := range line {
			r, c := p/_colCount, p%_colCount
			if s.symbolGrid[r][c] != _wild {
				allWild = false
				break
			}
		}
		if allWild {
			if odds := s.getSymbolBaseMultiplier(_wild, _minMatchCount); odds > 0 {
				var winGrid int64Grid
				for _, p := range line {
					r, c := p/_colCount, p%_colCount
					winGrid[r][c] = _wild
				}
				winInfos = append(winInfos, WinInfo{
					Symbol:      _wild,
					SymbolCount: _minMatchCount,
					LineCount:   int64(i),
					Odds:        odds,
					WinGrid:     winGrid,
				})
				for _, p := range line {
					r, c := p/_colCount, p%_colCount
					if winGrid[r][c] > 0 {
						totalWinGrid[r][c] = 1
					}
				}
			}
			continue
		}

		firstP := line[0]
		firstR := firstP / _colCount
		firstC := firstP % _colCount
		firstSymbol := s.symbolGrid[firstR][firstC]

		var (
			symbolCandidates [7]int64
			candCount        int
		)

		if firstSymbol == _wild {
			for symbol := _blank + 1; symbol < _wild; symbol++ {
				symbolCandidates[candCount] = symbol
				candCount++
			}
		} else if firstSymbol >= _blank+1 && firstSymbol < _wild {
			symbolCandidates[0] = firstSymbol
			candCount = 1
		} else {
			continue
		}

		for idx := 0; idx < candCount; idx++ {
			symbol := symbolCandidates[idx]
			var count int64
			var winGrid int64Grid

			for _, p := range line {
				r, c := p/_colCount, p%_colCount
				currSymbol := s.symbolGrid[r][c]
				if currSymbol == symbol || currSymbol == _wild {
					winGrid[r][c] = currSymbol
					count++
				} else {
					break
				}
			}

			if count >= _minMatchCount {
				if odds := s.getSymbolBaseMultiplier(symbol, int(count)); odds > 0 {
					winInfos = append(winInfos, WinInfo{
						Symbol:      symbol,
						SymbolCount: count,
						LineCount:   int64(i),
						Odds:        odds,
						WinGrid:     winGrid,
					})
					for _, p := range line {
						r, c := p/_colCount, p%_colCount
						if winGrid[r][c] > 0 {
							totalWinGrid[r][c] = 1
						}
					}
				}
			}
		}
	}

	s.winInfos = winInfos
	s.winGrid = totalWinGrid
}

// 检查是否九个位置全部为百搭
func (s *betOrderService) isAllWild() bool {
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			if s.symbolGrid[row][col] != _wild {
				return false
			}
		}
	}
	return true
}

// int64GridToArray 盘面转一维 9 格
func int64GridToArray(grid int64Grid) []int64 {
	out := make([]int64, _rowCount*_colCount)
	for row := 0; row < _rowCount; row++ {
		for col := 0; col < _colCount; col++ {
			out[row*_colCount+col] = grid[row][col]
		}
	}
	return out
}

func marshalProtoMessage(msg proto.Message) ([]byte, string, error) {
	pbData, err := proto.Marshal(msg)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := json.CJSON.MarshalToString(msg)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}
