// game/bxkh2/bet_order_step.go
package bxkh2

import (
	"strconv"
	"strings"
	"time"

	"egame-grpc/gamelogic"
	"egame-grpc/model/game"
	"egame-grpc/utils/json"

	"github.com/shopspring/decimal"
)

func (s *betOrderService) updateGameOrder() error {
	isFree := int64(0)
	if s.scene.IsFreeRound {
		isFree = 1
	}

	s.gameOrder = &game.GameOrder{
		MerchantID:    s.merchant.ID,
		Merchant:      s.merchant.Merchant,
		MemberID:      s.member.ID,
		Member:        s.member.MemberName,
		GameID:        s.game.ID,
		GameName:      s.game.GameName,
		BaseMultiple:  _baseMultiplier,
		Multiple:      s.req.Multiple,
		BonusMultiple: s.stepMultiplier,
		BaseAmount:    s.req.BaseMoney,
		Amount:        s.amount.Round(2).InexactFloat64(),
		ValidAmount:   s.amount.Round(2).InexactFloat64(),
		BonusAmount:   s.bonusAmount.Round(2).InexactFloat64(),
		CurBalance:    s.getCurrentBalance(),
		OrderSn:       s.orderSn.OrderSN,
		State:         1,
		HuNum:         s.scatterCount,
		FreeNum:       int64(s.client.ClientOfFreeGame.GetFreeNum()),
		FreeTimes:     int64(s.client.ClientOfFreeGame.GetFreeTimes()),
		IsFree:        isFree,
		CreatedAt:     time.Now().Unix(),
	}

	return s.fillOrderDetails()
}

func (s *betOrderService) getCurrentBalance() float64 {
	return decimal.NewFromFloat(s.member.Balance).
		Sub(s.amount).
		Add(s.bonusAmount).
		Round(2).
		InexactFloat64()
}

func (s *betOrderService) fillOrderDetails() error {
	betRaw, err := json.CJSON.MarshalToString(s.symbolGrid)
	if err != nil {
		return err
	}
	s.gameOrder.BetRawDetail = betRaw

	winRaw, err := json.CJSON.MarshalToString(s.winGrid)
	if err != nil {
		return err
	}
	s.gameOrder.BonusRawDetail = winRaw

	s.gameOrder.BetDetail = s.gridToString(s.symbolGrid)
	s.gameOrder.BonusDetail = s.gridToString(s.winGrid)

	winDetails := map[string]any{
		"sn":        s.gameOrder.OrderSn,
		"scatter":   s.scatterCount,
		"winGrid":   s.winGrid,
		"betAmt":    s.betAmount.Round(2).InexactFloat64(),
		"bnsAmt":    s.bonusAmount.Round(2).InexactFloat64(),
		"isFree":    s.scene.IsFreeRound,
		"totalFree": s.client.ClientOfFreeGame.GetFreeNum() + s.client.ClientOfFreeGame.GetFreeTimes(),
	}
	winDetailsStr, err := json.CJSON.MarshalToString(winDetails)
	if err != nil {
		return err
	}
	s.gameOrder.WinDetails = winDetailsStr
	return nil
}

func (s *betOrderService) gridToString(grid int64Grid) string {
	var b strings.Builder
	b.Grow(512)
	for i := 0; i < _rowCount; i++ {
		for j := 0; j < _colCount; j++ {
			b.WriteString(strconv.Itoa(i*_colCount + j + 1))
			b.WriteString(":")
			b.WriteString(strconv.FormatInt(grid[i][j], 10))
			b.WriteString("; ")
		}
	}
	return b.String()
}

func (s *betOrderService) settleStep() error {
	saveParam := &gamelogic.SaveTransferParam{
		Client:      s.client,
		GameOrder:   s.gameOrder,
		MerchantOne: s.merchant,
		MemberOne:   s.member,
		Ip:          s.req.Ip,
	}

	res := gamelogic.SaveTransfer(saveParam)
	return res.Err
}
