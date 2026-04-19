package ajtm

import (
	"time"

	"google.golang.org/protobuf/proto"

	"egame-grpc/game/ajtm/pb"
	"egame-grpc/game/common"
	"egame-grpc/gamelogic"
	"egame-grpc/global"
	"egame-grpc/model/game"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

func (s *betOrderService) initialize() error {
	switch {
	case !s.isFreeRound && s.scene.Steps == 0:
		return s.initFirstStepForSpin()
	default:
		return s.initStepForNextStep()
	}
}

func (s *betOrderService) initFirstStepForSpin() error {
	if s.debug.open {
		s.betAmount = decimal.NewFromInt(_baseMultiplier)
		s.amount = s.betAmount
		return nil
	}

	switch {
	case !s.updateBetAmount():
		return InvalidRequestParams
	case !s.checkBalance():
		return InsufficientBalance
	}
	s.scene.Reset()
	s.amount = s.betAmount
	return nil
}

func (s *betOrderService) initStepForNextStep() error {
	if s.debug.open {
		s.req.BaseMoney = 1
		s.req.Multiple = 1
		s.betAmount = decimal.NewFromInt(_baseMultiplier)
		s.amount = decimal.Zero
		return nil
	}

	s.req.BaseMoney = s.lastOrder.BaseAmount
	s.req.Multiple = s.lastOrder.Multiple
	s.betAmount = decimal.NewFromFloat(s.lastOrder.BaseAmount * float64(s.lastOrder.BaseMultiple*s.lastOrder.Multiple))
	s.amount = decimal.Zero
	return nil
}

func (s *betOrderService) updateGameOrder() error {
	orderSn := &common.OrderSN{}
	if !s.debug.open {
		isBaseStage := s.scene.Stage == _spinTypeBase
		isFreeStage := s.scene.Stage == _spinTypeFree || s.scene.Stage == _spinTypeFreeEli
		orderSn = common.GenerateOrderSn(s.member, s.lastOrder, isBaseStage, isFreeStage)
	}

	s.gameOrder = &game.GameOrder{
		MerchantID:        s.merchant.ID,
		Merchant:          s.merchant.Merchant,
		MemberID:          s.member.ID,
		Member:            s.member.MemberName,
		GameID:            s.game.ID,
		GameName:          s.game.GameName,
		BaseMultiple:      _baseMultiplier,
		Multiple:          s.req.Multiple,
		LineMultiple:      s.stepMultiplier,
		BonusHeadMultiple: 1,
		BonusMultiple:     1,
		BaseAmount:        s.req.BaseMoney,
		Amount:            s.amount.Round(2).InexactFloat64(),
		ValidAmount:       s.amount.Round(2).InexactFloat64(),
		BonusAmount:       s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:        decimal.NewFromFloat(s.member.Balance).Sub(s.amount).Add(s.bonusAmount).Round(2).InexactFloat64(),
		OrderSn:           orderSn.OrderSN,
		ParentOrderSn:     orderSn.ParentOrderSN,
		FreeOrderSn:       orderSn.FreeOrderSN,
		State:             1,
		BonusTimes:        1,
		HuNum:             s.scatterCount,
		FreeNum:           s.scene.FreeNum,
		FreeTimes:         s.scene.FreeTimes,
		CreatedAt:         time.Now().Unix(),
	}
	if s.isFreeRound {
		s.gameOrder.IsFree = 1
	}
	return s.fillInGameOrderDetails()
}

func (s *betOrderService) fillInGameOrderDetails() error {
	var err error
	if s.gameOrder.BetRawDetail, err = json.CJSON.MarshalToString(s.symbolGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal symbolGrid", zap.Error(err))
		return err
	}
	if s.gameOrder.BonusRawDetail, err = json.CJSON.MarshalToString(s.winGrid); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails: marshal winGrid", zap.Error(err))
		return err
	}
	if s.gameOrder.WinDetails, err = json.CJSON.MarshalToString(s.getWinDetails()); err != nil {
		global.GVA_LOG.Error("fillInGameOrderDetails", zap.Error(err))
		return err
	}
	return nil
}

func (s *betOrderService) getWinDetails() *WinDetails {
	/*result := &pb.Ajtm_BetOrderResponse{
		Sn:                 proto.String(s.orderSn.OrderSN),
		Balance:            proto.Float64(s.gameOrder.CurBalance),
		BetAmount:          proto.Float64(s.betAmount.Round(2).InexactFloat64()),
		CurWin:             proto.Float64(s.bonusAmount.Round(2).InexactFloat64()),
		FreeTotalWin:       proto.Float64(s.client.ClientOfFreeGame.GetFreeTotalMoney()),
		TotalWin:           proto.Float64(s.client.ClientOfFreeGame.GetGeneralWinTotal()),
		IsFree:             proto.Bool(s.isFreeRound),
		Review:             proto.Int64(s.req.Review),
		WinInfo:            s.buildWinInfo(),
		Cards:              s.int64GridToArray(s.symbolGrid),
		ScatterCount:       proto.Int64(s.scatterCount),
		IsRoundOver:        proto.Bool(s.isRoundOver),
		State:              proto.Int64(int64(s.scene.Stage)),
		RemainingFreeTimes: proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeNum())),
		TotalFreeTimes:     proto.Int64(int64(s.client.ClientOfFreeGame.GetFreeTimes())),
		StepMul:            proto.Int64(s.stepMultiplier),
		WinGrid:            s.int64GridToArray(s.winGrid),
		IsGameOver:         proto.Bool(s.isFreeRound && s.isRoundOver && s.scene.FreeNum <= 0),
		RoundWin:           proto.Float64(s.calcRoundWin()),
		MysMul:             proto.Int64(s.mysMul),
		WinMys:             s.buildWinMys(),
	}*/

	winArr := make([]*pb.Ajtm_WinArr, len(s.winInfos))
	for i, elem := range s.winInfos {
		winArr[i] = &pb.Ajtm_WinArr{
			RoadNum: proto.Int64(elem.LineCount),
			Odds:    proto.Int64(elem.Odds),
		}
	}

	events := make([]*pb.Ajtm_WinMys, len(s.winMys))
	for i, event := range s.winMys {
		events[i] = &pb.Ajtm_WinMys{
			Col:       proto.Int64(event.Col),
			HeadRow:   proto.Int64(event.HeadRow),
			TailRow:   proto.Int64(event.TailRow),
			OldSymbol: proto.Int64(event.OldSymbol),
			NewSymbol: proto.Int64(event.NewSymbol),
		}
	}

	return &WinDetails{
		FreeWin:      s.scene.FreeWin,
		RoundWin:     s.scene.RoundWin,
		IsRoundOver:  s.isRoundOver,
		State:        int64(s.scene.Stage),
		NewFreeTimes: s.addFreeTime,
		WinArr:       winArr,
		WinMys:       events,
		MysMul:       s.mysMul,
		Limit:        s.limit,
	}
}

/*
	func (s *betOrderService) buildWinInfo() *pb.Ajtm_WinInfo {
		winArr := make([]*pb.Ajtm_WinArr, len(s.winInfos))
		for i, elem := range s.winInfos {
			winArr[i] = &pb.Ajtm_WinArr{
				RoadNum: proto.Int64(elem.LineCount),
				Odds:    proto.Int64(elem.Odds),
			}
		}
		return &pb.Ajtm_WinInfo{
			WinArr:     winArr,
			AddFreeNum: proto.Int64(s.addFreeTime),
			Limit:      proto.Bool(s.limit),
		}
	}

	func (s *betOrderService) buildWinMys() []*pb.Ajtm_WinMys {
		events := make([]*pb.Ajtm_WinMys, len(s.winMys))
		for i, event := range s.winMys {
			events[i] = &pb.Ajtm_WinMys{
				Col:       proto.Int64(event.Col),
				HeadRow:   proto.Int64(event.HeadRow),
				TailRow:   proto.Int64(event.TailRow),
				OldSymbol: proto.Int64(event.OldSymbol),
				NewSymbol: proto.Int64(event.NewSymbol),
			}
		}
		return events
	}

	func (s *betOrderService) calcRoundWin() float64 {
		if s.scene.RoundMultiplier == 0 {
			return 0
		}
		return decimal.NewFromFloat(s.req.BaseMoney).
			Mul(decimal.NewFromInt(s.req.Multiple)).
			Mul(decimal.NewFromInt(s.scene.RoundMultiplier)).
			Round(2).InexactFloat64()
	}
*/
func (s *betOrderService) settleStep() error {
	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}
	return gamelogic.SaveTransfer(saveParam).Err
}
