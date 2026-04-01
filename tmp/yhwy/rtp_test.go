package yhwy

import (
	"testing"

	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/shopspring/decimal"
)

func TestMysteryFlowSmoke(t *testing.T) {
	svc := newBerService()
	svc.initGameConfigs()

	for i := 0; i < 50; i++ {
		if err := svc.baseSpin(); err != nil {
			t.Fatalf("baseSpin failed: %v", err)
		}
		if svc.spreadToReel < 1 || svc.spreadToReel > _colCount {
			t.Fatalf("invalid spreadToReel: %d", svc.spreadToReel)
		}
		if svc.isSakuraReset && svc.resetDirection == _resetDirectionNone {
			t.Fatalf("reset triggered without direction")
		}
		if svc.hasMysteryOnBoard() && svc.revealSymbol == _blank {
			t.Fatalf("mystery exists without reveal symbol")
		}
		if svc.isRoundOver && svc.scene.FreeNum <= 0 {
			resetBetServiceForNextRound(svc)
		}
	}
}

func newBerService() *betOrderService {
	return &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     GameID,
			BaseMoney:  1,
			Multiple:   1,
		},
		merchant: &merchant.Merchant{
			ID:       20020,
			Merchant: "TestMerchant",
		},
		member: &member.Member{
			ID:         1,
			MemberName: "TestUser",
			Balance:    10000000,
			Currency:   "USD",
		},
		game: &game.Game{
			ID: GameID,
		},
		client: &client.Client{
			ClientOfFreeGame: &client.ClientOfFreeGame{},
			ClientGameCache:  &client.ClientGameCache{},
		},
		scene:       &SpinSceneData{},
		bonusAmount: decimal.Decimal{},
		betAmount:   decimal.Decimal{},
		amount:      decimal.Decimal{},
		debug:       rtpDebugData{open: true},
	}
}

func resetBetServiceForNextRound(s *betOrderService) {
	s.stepMultiplier = 0
	s.scatterCount = 0
	s.isFreeRound = false
	s.scene = &SpinSceneData{}
	s.client.IsRoundOver = false
	s.client.ClientOfFreeGame.Reset()
	s.client.ClientOfFreeGame.ResetGeneralWinTotal()
	s.client.ClientOfFreeGame.ResetRoundBonus()
	s.client.SetLastMaxFreeNum(0)
}
