package gcd

import (
	"egame-grpc/game/common"
	"egame-grpc/game/common/rand"
	"egame-grpc/game/gcd/pb"
	"egame-grpc/gamelogic"
	"egame-grpc/model/game"
	"egame-grpc/utils/jsonx"
	"errors"
	redisv8 "github.com/go-redis/redis/v8"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/proto"
)

func (s *betOrderService) getRequestContext() error {
	mer, mem, ga, err := common.GetRequestContext(s.req)
	if err != nil {
		return err
	}
	s.merchant = mer
	s.member = mem
	s.game = ga
	return nil
}

func (s *betOrderService) checkBalance() bool {
	f, _ := s.betAmount.Float64()
	return gamelogic.CheckMemberBalance(f, s.member)
}

func (s *betOrderService) calculateTrigger(featureRate []int64) bool {
	if len(featureRate) < 2 || featureRate[1] <= 0 {
		return false
	}
	r := rand.IntN(int(featureRate[1])) + 1
	return r <= int(featureRate[0])
}

func countSymbol(grid Int64Grid, symbol int64) int64 {
	num := int64(0)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			if grid[r][c] == symbol {
				num += 1
			}
		}
	}
	return num
}

func getValByWeight(val []int64, weight []int) int64 {
	total := 0
	for _, n := range weight {
		total += n
	}
	if total <= 0 || len(val) == 0 {
		if len(val) > 0 {
			return val[0]
		}
		return 0
	}
	r := rand.IntN(total)
	index := 0
	for i, w := range weight {
		if r < w {
			index = i
			break
		}
		r -= w
	}
	return val[index]
}

func int64GridToPbBoard(grid Int64Grid) *pb.Board {
	elements := make([]int64, _rowCount*_colCount)
	for row := int64(0); row < _rowCount; row++ {
		for col := int64(0); col < _colCount; col++ {
			elements[row*_colCount+col] = grid[row][col]
		}
	}
	return &pb.Board{Elements: elements}
}

func marshalData(data proto.Message) ([]byte, string, error) {
	pbData, err := proto.Marshal(data)
	if err != nil {
		return nil, "", err
	}
	jsonData, err := jsonx.MarshalString(data)
	if err != nil {
		return nil, "", err
	}
	return pbData, jsonData, nil
}

func isNilFromCache(err error) bool {
	if errors.Is(err, redisv8.Nil) {
		return true
	}
	if errors.Is(err, redisv9.Nil) {
		return true
	}
	return false
}

func gameOrderToResponse(gameOrder *game.GameOrder) (*pb.Gcd_BetOrderResponse, error) {
	winDetail := new(WinDetails)
	if err := jsonx.UnmarshalString(gameOrder.WinDetails, winDetail); err != nil {
		return nil, err
	}
	symbolGrid := Int64Grid{}
	if err := jsonx.UnmarshalString(gameOrder.BetRawDetail, &symbolGrid); err != nil {
		return nil, err
	}
	isFree := gameOrder.IsFree == 1
	isSpinOver := winDetail.NextStage == _normalMode
	totalFreeTimes := winDetail.FreeNum + winDetail.FreeTimes
	winCards := &pb.Board{Elements: make([]int64, _rowCount*_colCount)}
	for _, row := range winDetail.WinArr {
		for i, e := range row.Grid.Elements {
			if e > 0 {
				winCards.Elements[i] = e
			}
		}
	}
	betAmount := decimal.NewFromFloat(gameOrder.BaseAmount * float64(gameOrder.BaseMultiple) * float64(gameOrder.Multiple)).Round(2).InexactFloat64()
	return &pb.Gcd_BetOrderResponse{
		OrderSn:            &gameOrder.OrderSn,
		Balance:            &gameOrder.CurBalance,
		BaseBet:            &gameOrder.BaseAmount,
		BetAmount:          &betAmount,
		Multiplier:         &gameOrder.Multiple,
		CurWin:             &gameOrder.BonusAmount,
		RoundWin:           &winDetail.RoundWin,
		TotalWin:           &winDetail.TotalWin,
		FreeTotalWin:       &winDetail.FreeWin,
		IsRoundOver:        &winDetail.IsRoundOver,
		IsSpinOver:         &isSpinOver,
		IsFree:             &isFree,
		NewFreeTimes:       &winDetail.NewFreeTimes,
		RemainingFreeTimes: &winDetail.FreeNum,
		TotalFreeTimes:     &totalFreeTimes,
		Cards:              int64GridToPbBoard(symbolGrid),
		WinCards:           winCards,
		GameInfo: &pb.Gcd_GameInfo{
			RoundStep: &winDetail.StepIndex,
			WinArr:    winDetail.WinArr,
		},
		FreeType:   &winDetail.FreeType,
		BonusState: &winDetail.BonusState,
	}, nil
}
