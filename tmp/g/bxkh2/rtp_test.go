package bxkh2

import (
	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"egame-grpc/utils"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

var points = []int64{500, 1000, 5000, 10000}

const rtp = 968

func TestRtp(t *testing.T) {

	betService := newBerService()

	runtime := int64(0)
	totalRuntime := int64(10000000)

	var totalWin, freeWin, baseWin, baseWinTime, freeTime, freeRound, freeWinTime, stepMultiplier int64

	var spinMultiplier = int64(0)

	var tmpRtpSlice = make([][]string, len(points))
	var tmpRtpCount = make([]int, len(points))
	for i := 0; i < len(points); i++ {
		tmpRtpSlice[i] = make([]string, totalRuntime/points[i])
	}

	var head []string
	for _, point := range points {
		head = append(head, fmt.Sprintf("base-%d,free-%d,total-%d", point, point, point))
	}

	header := strings.Join(head, ",")
	fmt.Println()
	betService.initGameConfigs()

	for {

		betService.gameConfig = _gameJsonConfig
		betService.scene.Stage = _spinTypeBase
		if betService.scene.IsFreeRound {
			betService.scene.Stage = _spinTypeFree
		}

		//新免费回合开始
		if betService.scene.NextStage > 0 && betService.scene.NextStage != betService.scene.Stage {
			betService.scene.Stage = betService.scene.NextStage
			betService.scene.NextStage = 0
		}

		if betService.scene.Stage == _spinTypeFree {
			betService.scene.IsFreeRound = true
		}

		if betService.scene.Stage == _spinTypeBase {
			betService.scene.IsFreeRound = false
		}

		err := betService.baseSpin()
		if err != nil {
			panic(err)
		}

		// 获取当前步骤倍数
		currentStepMult := betService.scene.SpinMultiplier
		stepMultiplier += currentStepMult
		totalWin += currentStepMult
		spinMultiplier += currentStepMult

		if betService.scene.IsFreeRound {
			freeWin += currentStepMult
			if betService.scene.RoundOver {
				freeRound++
				if betService.scene.RoundMultiplier > 0 {
					freeWinTime++
				}
			}
		} else {
			baseWin += currentStepMult
			if betService.scene.RoundMultiplier > 0 && betService.scene.RoundOver {
				baseWinTime++
			}
		}

		if (betService.scene.Stage == _spinTypeBase || betService.scene.Stage == _spinTypeBaseEli) && betService.client.ClientOfFreeGame.GetFreeNum() > 0 {
		}

		// 回合结束
		if betService.scene.RoundOver {
			if !betService.scene.IsFreeRound {
			}
			runtime++
			if betService.scene.Stage == _spinTypeFree || betService.scene.Stage == _spinTypeFreeEli {
				freeTime++
			}
			spinMultiplier = 0
			betService = newBerService()
			if runtime%10000 == 0 {
				tmpRtpSlice[3][tmpRtpCount[3]] = fmt.Sprintf("%.4f,%.4f,%.4f",
					decimal.NewFromInt(baseWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(freeWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(totalWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
				)
				tmpRtpCount[3]++
			}

			if runtime%1000000 == 0 {
				if freeRound == 0 {
					freeRound = 1
				}
				fmt.Printf("\rRuntime-%d baseRtp=%.4f%%,baseWinRate-%.4f%% freeRtp-%.4f%% freeWinRate-%.4f%%, freeTriggerRate-%.4f%% Rtp-%.4f%%\n",
					runtime,
					decimal.NewFromInt(baseWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(baseWinTime).Div(decimal.NewFromInt(runtime)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(freeWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(freeWinTime).Div(decimal.NewFromInt(freeRound)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(freeTime).Div(decimal.NewFromInt(runtime)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(totalWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
				)
				fmt.Printf("\rtotalWin-%d freeWin=%d,baseWin-%d ,baseWinTime-%d ,freeTime-%d, freeRound-%d ,freeWinTime-%d,freeNum-%d\n",
					totalWin, freeWin, baseWin, baseWinTime, freeTime, freeRound, freeWinTime, betService.client.ClientOfFreeGame.GetFreeNum())
				stepMultiplier = 0

			}
		}

		if runtime == totalRuntime {
			break
		}

	}
	return

	fp, err := os.OpenFile(fmt.Sprintf("%d-%d.csv", _gameID, rtp), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
	if err != nil {
		panic(err)
	}
	defer fp.Close()
	fp.WriteString(header)
	fp.WriteString("\n")
	for k, s := range tmpRtpSlice[0] {
		line := s
		for l := 1; l < len(points); l++ {
			if k < len(tmpRtpSlice[l])-1 {
				line = fmt.Sprintf("%s,%s", line, tmpRtpSlice[l][k])
			}
		}
		fp.WriteString(line)
		fp.WriteString("\n")
	}
}

func newBerService() *betOrderService {
	s := &betOrderService{
		req: &request.BetOrderReq{
			MerchantId: 20020,
			MemberId:   1,
			GameId:     18898,
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
			ID: 18898,
		},
		client:    client.NewClient(&utils.Token{MemberId: 1, Member: "Jack23"}),
		lastOrder: nil,
		scene:     &SpinSceneData{},
		gameOrder: nil,
		debug:     rtpDebugData{open: true},
	}
	s.initGameConfigs()
	return s
}
