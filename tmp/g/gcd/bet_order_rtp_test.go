package gcd

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"

	"github.com/shopspring/decimal"
)

var testConfig *GameConfig

var totalWin, freeWin_base, freeWin_eli, baseWin, baseWinTime, freeTime, freeRound, freeWinTime int64
var spinMultiplier int64
var runtime int64
var selectType bool
var freeType int64 = 3

func TestRtp(t *testing.T) {
	clearDebugBuffer()
	loadDebugConf()
	roundId := 0
	totalRuntime := rtpTimes
	s := newBetService()
	for {
		if err := s.ensureBonusSelected(); err != nil {
			autoSelectRtpBonus(t, s)
			continue
		}

		refreshData(s)
		roundId += 1
		if err := s.stepScene(); err != nil {
			panic(err)
		}
		if debugLogging {
			debugPrintln()
			debugPrintln()
		}
		debugPrintTips(fmt.Sprintf("第 %d 次开始（%d）", runtime+1, roundId))
		debugPrintTips(fmt.Sprintf("当前阶段：%s", stepTranMap[s.scene.Stage]))
		if err := s.betSpins(); err != nil {
			panic(err)
		}
		debugTimes += 1

		//if s.scene.NextStage == _freeMode && !selectType {
		//	s.scene.FreeType = int64(freeType)
		//	times := s.gameConfig.GetFreeCfgByType(int64(freeType)).Times
		//	s.scene.FreeNum += times
		//	s.client.SetMaxFreeNum(uint64(times))
		//	debugPrintTips(fmt.Sprintf("触发免费,免费次数 %d", times))
		//	selectType = true
		//}

		stepMul := s.stepMultiplier
		isFree := s.isFreeMode()
		totalWin += stepMul
		spinMultiplier += stepMul
		if isFree {
			if s.scene.Stage == _freeMode {
				freeWin_base += stepMul
			} else {
				freeWin_eli += stepMul
			}
			freeTime++
		} else {
			baseWin += stepMul
		}
		if debugLogging {
			debugPrintTips(fmt.Sprintf("回合倍率：+%d，中奖倍率：%d，累计倍率：%d", s.roundMulti, s.stepMultiplier, spinMultiplier))
			debugPrintTips(fmt.Sprintf("下个阶段：%s", stepTranMap[s.scene.NextStage]))
		}

		if s.scene.NextStage == _normalMode {
			runtime++
			if spinMultiplier > 0 {
				if isFree {
					freeWinTime++
				} else {
					baseWinTime++
				}
			}
			if isFree {
				freeRound++
			}
			if debugLogging {
				debugPrintln()
				debugPrintTips(fmt.Sprintf("第 %d 次spin结束，当次总赢金额：%d", runtime, spinMultiplier))
				debugPrintTips(fmt.Sprintf("累计普通金额：%d，累计免费金额：%d，累计总中奖金额：%d", baseWin, freeWin_base+freeWin_eli, totalWin))
				debugPrintTips("")
			}
			s = newBetService()
			roundId = 0
			spinMultiplier = 0
			selectType = false
			if runtime%1000000 == 0 {
				printfResult()
			}
		}
		if runtime == totalRuntime {
			printfResult()
			break
		}
	}
}

func printfResult() {
	freeRoundForPrint := freeRound
	if freeRoundForPrint == 0 {
		freeRoundForPrint = 1
	}
	reqMultiplier := runtime * _baseMultiplier
	debugPrintln()
	debugPrintln(fmt.Sprintf("下注次数-%d，下注额-%d", runtime, reqMultiplier))
	debugPrintln(fmt.Sprintf("普通总赢-%d, 普通赢次数-%d, 普通赢率-%.8f, 普通Rtp-%.8f,", baseWin, baseWinTime,
		decimal.NewFromInt(baseWinTime).Div(decimal.NewFromInt(runtime)).Round(8).InexactFloat64(),
		decimal.NewFromInt(baseWin).Div(decimal.NewFromInt(reqMultiplier)).Round(8).InexactFloat64(),
	))
	freeTimeTmp := freeTime
	if freeTimeTmp == 0 {
		freeTimeTmp = 1
	}
	debugPrintln(fmt.Sprintf("免费总赢-%d,免费基础总赢-%d,免费消除总赢-%d, 触发免费次数-%d ,免费总次数-%d, 免费赢次数-%d, 免费触发率-%.8f, 免费中免费触发率-%.8f,  免费赢率-%.8f, 免费Rtp-%.8f",
		freeWin_base+freeWin_eli, freeWin_base, freeWin_eli, freeRound, freeTime, freeWinTime,
		decimal.NewFromInt(freeRound).Div(decimal.NewFromInt(runtime)).Round(8).InexactFloat64(),
		decimal.NewFromInt(freeInFree).Div(decimal.NewFromInt(freeTimeTmp)).Round(8).InexactFloat64(),
		decimal.NewFromInt(freeWinTime).Div(decimal.NewFromInt(freeRoundForPrint)).Round(8).InexactFloat64(),
		decimal.NewFromInt(freeWin_base+freeWin_eli).Div(decimal.NewFromInt(reqMultiplier)).Round(8).InexactFloat64(),
	))
	debugPrintln(fmt.Sprintf("总赢-%d ,Rtp-%.8f", totalWin,
		decimal.NewFromInt(totalWin).Div(decimal.NewFromInt(reqMultiplier)).Round(8).InexactFloat64(),
	))
	debugPrintln()
}

func newBetService() *betOrderService {
	s := &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     GameID,
			BaseMoney:  1,
			Multiple:   1,
			Purchase:   0,
			Review:     0,
			Merchant:   "Jack23",
			Member:     "Jack23",
		},
		merchant: &merchant.Merchant{
			ID:       20020,
			Merchant: "Jack23",
		},
		member: &member.Member{
			ID:         1,
			MemberName: "Jack23",
			Balance:    10000000,
			Currency:   "USD",
		},
		game: &game.Game{
			ID: GameID,
		},
		client: &client.Client{
			MemberId:   1,
			Member:     "Jack23",
			GameId:     GameID,
			Timestamp:  time.Now().Unix(),
			ActivityId: 0,
			Lock:       sync.Mutex{},
			BetLock:    sync.Mutex{},
			SyncLock:   sync.Mutex{},
			ClientOfGame: &client.ClientOfGame{
				HeadMultipleTimes: []uint64{},
				TimeList:          []int{},
				Lock:              sync.Mutex{},
			},
			SliceSlow: []int64{},
			ClientOfFreeGame: &client.ClientOfFreeGame{
				Lock: sync.Mutex{},
			},
			ClientGameCache: &client.ClientGameCache{
				ExpiredTime: 90 * 24 * 3600 * time.Second,
			},
		},
		lastOrder: &game.GameOrder{
			BaseAmount: 1,
			Multiple:   1,
		},
		scene:          &SpinSceneData{NextStage: _normalMode},
		gameOrder:      nil,
		bonusAmount:    decimal.Decimal{},
		betAmount:      decimal.Decimal{},
		amount:         decimal.Decimal{},
		stepMultiplier: 0,
	}

	if testConfig == nil {
		testConfig = newGameConfig("")
	}
	s.gameConfig = testConfig
	return s
}

func refreshData(s *betOrderService) {
	s.winResults = nil
	s.winArr = nil
	s.symbolGrid = Int64Grid{}
	s.winGrid = Int64Grid{}
	s.freeType = 0
	s.roundMulti = 0
	s.lineMultiplier = 0
	s.stepMultiplier = 0
}

func autoSelectRtpBonus(t *testing.T, s *betOrderService) {
	t.Helper()
	freeNum, err := s.selectFreeBonus(freeType)
	if err != nil {
		t.Fatalf("rtp auto select bonus failed: scatter=%d err=%v", s.scene.ScatterNum, err)
	}
	s.newFreeTimes = freeNum
}
