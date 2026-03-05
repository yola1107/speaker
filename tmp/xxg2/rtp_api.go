//package xxg2
//
//import (
//	"fmt"
//	"log/slog"
//
//	"egame-grpc/game/common"
//	"egame-grpc/game/common/rtp_debug_utils"
//	"egame-grpc/global/client"
//	"egame-grpc/model/game"
//	"egame-grpc/model/game/request"
//	"egame-grpc/model/member"
//	"egame-grpc/model/merchant"
//
//	"github.com/shopspring/decimal"
//)
//
//var closed bool
//
//type XXGExporter struct{}
//
//func NewExporter() *XXGExporter {
//	return &XXGExporter{}
//}
//
//func (e *XXGExporter) Name() string {
//	return "吸血鬼"
//}
//
//func (e *XXGExporter) Stop() {
//	closed = true
//}
//
//func (e *XXGExporter) DebugRtp(req *request.RtpDebug, ch chan rtp_debug_utils.RtpResult) {
//	slog.Info("xxg2 rtp debug start", "req", req)
//	initGameConfigs()
//	closed = false
//
//	baseMoney := req.BaseMoney
//	if baseMoney <= 0 {
//		baseMoney = 1
//	}
//	multiple := req.Multiple
//	if multiple <= 0 {
//		multiple = 1
//	}
//
//	chunk := req.Chunk
//	if chunk <= 0 {
//		chunk = req.Times
//		if chunk <= 0 {
//			chunk = 1
//		}
//	}
//
//	pool := common.NewGrPool(1000, int(req.Times))
//	resCh := make(chan rtp_debug_utils.SpinResult, 128)
//
//	rtpResult := rtp_debug_utils.RtpResult{
//		TotalTime: req.Times,
//	}
//
//	betAmount := baseMoney * float64(multiple) * float64(_cnf.BaseBat)
//
//	var (
//		freeWinRound  int64
//		baseWinRound  int64
//		formatPercent = func(num, den float64) string {
//			if den <= 0 {
//				return "0.0000"
//			}
//			return fmt.Sprintf("%.4f", num/den*100)
//		}
//	)
//
//	go func() {
//		for res := range resCh {
//			rtpResult.TotalWin += res.Win
//			rtpResult.TotalBet += betAmount
//			rtpResult.SpinTime++
//			rtpResult.StepTimes += res.Step
//			rtpResult.FreeRounds += res.FreeRound
//			rtpResult.FreeSteps += res.FreeStep
//			rtpResult.BaseWin += res.BaseWin
//			rtpResult.FreeWin += res.FreeWin
//			rtpResult.FreeInFreeTime += res.FreeInFree
//
//			freeWinRound += res.FreeWinRound
//			if res.BaseWin > 0 {
//				baseWinRound++
//			}
//			if res.FreeRound > 0 {
//				rtpResult.FreeTriggerTime++
//			}
//
//			if rtpResult.SpinTime%chunk == 0 {
//				rtpResult.RTP = formatPercent(rtpResult.TotalWin, rtpResult.TotalBet)
//				rtpResult.BaseRTP = formatPercent(rtpResult.BaseWin, rtpResult.TotalBet)
//				rtpResult.FreeRtp = formatPercent(rtpResult.FreeWin, rtpResult.TotalBet)
//				rtpResult.FreeRoundWinRate = formatPercent(float64(freeWinRound), float64(rtpResult.FreeRounds))
//				rtpResult.FreeTriggerRate = formatPercent(float64(rtpResult.FreeTriggerTime), float64(rtpResult.SpinTime))
//				rtpResult.BaseRoundWinRate = formatPercent(float64(baseWinRound), float64(rtpResult.SpinTime))
//				rtpResult.FreeInFreeRate = formatPercent(float64(rtpResult.FreeInFreeTime), float64(rtpResult.FreeTriggerTime))
//				rtpResult.Stop = rtpResult.SpinTime >= rtpResult.TotalTime || closed
//				ch <- rtpResult
//			}
//
//			pool.Done()
//		}
//
//		if rtpResult.SpinTime%chunk != 0 && rtpResult.SpinTime > 0 {
//			rtpResult.RTP = formatPercent(rtpResult.TotalWin, rtpResult.TotalBet)
//			rtpResult.BaseRTP = formatPercent(rtpResult.BaseWin, rtpResult.TotalBet)
//			rtpResult.FreeRtp = formatPercent(rtpResult.FreeWin, rtpResult.TotalBet)
//			rtpResult.FreeRoundWinRate = formatPercent(float64(freeWinRound), float64(rtpResult.FreeRounds))
//			rtpResult.FreeTriggerRate = formatPercent(float64(rtpResult.FreeTriggerTime), float64(rtpResult.SpinTime))
//			rtpResult.BaseRoundWinRate = formatPercent(float64(baseWinRound), float64(rtpResult.SpinTime))
//			rtpResult.FreeInFreeRate = formatPercent(float64(rtpResult.FreeInFreeTime), float64(rtpResult.FreeTriggerTime))
//			rtpResult.Stop = true
//			ch <- rtpResult
//		}
//	}()
//
//	var totalSpinTime int64
//	for totalSpinTime < req.Times {
//		if closed {
//			break
//		}
//		totalSpinTime++
//		pool.Add(func() {
//			spinRes := simulateSpin(req, baseMoney, multiple)
//			resCh <- spinRes
//			pool.Finish()
//		})
//	}
//
//	pool.Wait()
//	close(resCh)
//	slog.Info("xxg2 rtp debug finish")
//}
//
//func simulateSpin(req *request.RtpDebug, baseMoney float64, multiple int64) rtp_debug_utils.SpinResult {
//	var spinRes rtp_debug_utils.SpinResult
//
//	defer func() {
//		if r := recover(); r != nil {
//			slog.Error("xxg2 rtp spin panic", "error", r)
//		}
//	}()
//
//	svc := newGameForRtp(req, baseMoney, multiple)
//
//	for {
//		if closed {
//			return spinRes
//		}
//
//		updateSceneForRtp(svc)
//
//		res, err := svc.baseSpin()
//		if err != nil {
//			slog.Error("xxg2 rtp spin error", "error", err)
//			return spinRes
//		}
//
//		spinRes.Step++
//
//		isFree := svc.isFreeRound()
//		if isFree {
//			spinRes.FreeStep++
//			spinRes.FreeRound++
//		}
//
//		winAmount := float64(res.StepMultiplier) * baseMoney * float64(multiple)
//		if winAmount > 0 {
//			spinRes.Win += winAmount
//			if isFree {
//				spinRes.FreeWin += winAmount
//				spinRes.FreeWinRound++
//			} else {
//				spinRes.BaseWin += winAmount
//			}
//		}
//
//		if isFree && svc.newFreeCount > 0 {
//			spinRes.FreeInFree++
//		}
//
//		if res.SpinOver {
//			spinRes.Round = 1
//			return spinRes
//		}
//	}
//}
//
//func newGameForRtp(req *request.RtpDebug, baseMoney float64, multiple int64) *betOrderService {
//	svc := newBetOrderService(true)
//
//	merchantName := req.Merchant
//	if merchantName == "" {
//		merchantName = "RTPMerchant"
//	}
//	memberName := req.Member
//	if memberName == "" {
//		memberName = "RTPMember"
//	}
//
//	betReq := &request.BetOrderReq{
//		MerchantId: 1,
//		MemberId:   1,
//		GameId:     _gameID,
//		BaseMoney:  baseMoney,
//		Multiple:   multiple,
//		Merchant:   merchantName,
//		Member:     memberName,
//	}
//
//	svc.req = betReq
//	svc.merchant = &merchant.Merchant{
//		ID:       1,
//		Merchant: merchantName,
//	}
//	svc.member = &member.Member{
//		ID:         1,
//		MemberName: memberName,
//		Balance:    10000000,
//		Currency:   "USD",
//	}
//	svc.game = &game.Game{
//		ID:       _gameID,
//		GameName: "XXG2",
//	}
//	svc.client = &client.Client{
//		MemberId:         1,
//		Member:           memberName,
//		Merchant:         merchantName,
//		GameId:           _gameID,
//		ClientOfFreeGame: &client.ClientOfFreeGame{},
//		ClientGameCache:  &client.ClientGameCache{},
//	}
//	svc.scene = &SpinSceneData{}
//	svc.bonusAmount = decimal.Zero
//	svc.betAmount = decimal.NewFromFloat(baseMoney).
//		Mul(decimal.NewFromInt(multiple)).
//		Mul(decimal.NewFromInt(_cnf.BaseBat))
//	svc.amount = decimal.Zero
//	svc.lineMultiplier = 0
//	svc.stepMultiplier = 0
//	svc.newFreeCount = 0
//	svc.winInfos = nil
//	svc.winResults = nil
//	svc.symbolGrid = nil
//	svc.winGrid = nil
//	svc.stepMap = nil
//	svc.debug = rtpDebugData{open: true}
//
//	return svc
//}
//
//func updateSceneForRtp(s *betOrderService) {
//	if s.client.ClientOfFreeGame.GetFreeNum() > 0 {
//		s.scene.Stage = _spinTypeFree
//	} else {
//		s.scene.Stage = _spinTypeBase
//	}
//
//	if s.scene.NextStage > 0 && s.scene.NextStage != s.scene.Stage {
//		s.scene.Stage = s.scene.NextStage
//		s.scene.NextStage = 0
//	}
//}

package xxg2
