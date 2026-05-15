package ycpd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"egame-grpc/global/client"
	"egame-grpc/model/game"
	"egame-grpc/model/game/request"
	"egame-grpc/model/member"
	"egame-grpc/model/merchant"
	"github.com/shopspring/decimal"
)

var points = []int64{500, 1000, 5000, 10000}

const rtp = 968

type GameTestData struct {
	RowRandomIndex  [5]int
	CurrentTotalWin int
	gameMultiple    int
	stepMultiplier  int
	IsFirstTimes    bool
}

type GameTestDataSet []GameTestData

var GameTestDataInfo GameTestData

func TestRtp(t *testing.T) {
	betService := newRtpBetService()

	runtime := int64(0)
	totalRuntime := int64(100000000)
	if v := os.Getenv("YCPD_RTP_TOTAL_RUNTIME"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			t.Fatalf("invalid YCPD_RTP_TOTAL_RUNTIME: %s", v)
		}
		totalRuntime = n
	}

	var totalWin, freeWin, baseWin, baseWinTime, freeTime, freeRound, freeWinTime, stepMultiplier int64

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

	parseGameConfigs(_gameJsonConfigsRaw)

	for {
		betService.gameConfig = _gameJsonConfig
		betService.syncGameStage()

		if err := betService.baseSpin(); err != nil {
			panic(err)
		}

		totalWin += betService.stepMultiplier
		stepMultiplier += betService.stepMultiplier

		if betService.isFreeRound {
			freeWin += betService.stepMultiplier
			if betService.isRoundOver {
				freeRound++
				if stepMultiplier > 0 {
					freeWinTime++
				}
			}
		} else {
			baseWin += betService.stepMultiplier
			if stepMultiplier > 0 && betService.isRoundOver {
				baseWinTime++
			}
		}

		if betService.isRoundOver {
			stepMultiplier = 0
		}
		if betService.isSpinOver {
			runtime++
			if betService.isFreeRound {
				freeTime++
			}

			betService = newRtpBetService()

			if false {
				if runtime%10000 == 0 {
					tmpRtpSlice[3][tmpRtpCount[3]] = fmt.Sprintf("%.4f,%.4f,%.4f",
						decimal.NewFromInt(baseWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
						decimal.NewFromInt(freeWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
						decimal.NewFromInt(totalWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					)
					tmpRtpCount[3]++
				}
			}
			if runtime%1000000 == 0 {
				if freeRound == 0 {
					freeRound = 1
				}
				fmt.Printf("\r总次数-%d 普通Rtp=%.4f%%,普通赢奖率-%.4f%% 免费Rtp-%.4f%% 免费赢奖率-%.4f%%, 免费触发率-%.4f%% 总Rtp-%.4f%%\n",
					runtime,
					decimal.NewFromInt(baseWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(baseWinTime).Div(decimal.NewFromInt(runtime)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(freeWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(freeWinTime).Div(decimal.NewFromInt(freeRound)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(freeTime).Div(decimal.NewFromInt(runtime)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
					decimal.NewFromInt(totalWin).Div(decimal.NewFromInt(runtime*20)).Mul(decimal.NewFromInt(100)).Round(4).InexactFloat64(),
				)
				fmt.Printf("\r总赢-%d 免费总赢=%d,普通总赢-%d ,普通赢次数-%d ,免费触发次数-%d, 免费总次数-%d ,免费赢次数-%d\n",
					totalWin, freeWin, baseWin, baseWinTime, freeTime, freeRound, freeWinTime)
			}
		}

		if runtime == totalRuntime {
			break
		}
	}

	if false {
		fp, err := os.OpenFile(fmt.Sprintf("%d-%d.csv", GameID, rtp), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0700)
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
}

func newRtpBetService() *betOrderService {
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

func ParseImportData() (GameTestDataSet, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	filePath := filepath.Join(dir, "importData.txt")

	fileHandle, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %v", err)
	}
	defer fileHandle.Close()

	dataSet := make(GameTestDataSet, 0)
	var currentData *GameTestData
	var dataFieldsFilled int
	isFirstTimes := true

	scanner := bufio.NewScanner(fileHandle)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || strings.Contains(line, "PASS") || strings.Contains(line, "Runtime") {
			continue
		}

		if strings.HasPrefix(line, "基础模式第") {
			currentData = nil
			dataFieldsFilled = 0
			isFirstTimes = true
		}

		if currentData == nil {
			currentData = &GameTestData{}
			dataFieldsFilled = 0
			continue
		}

		if strings.HasPrefix(line, "初始索引-") && dataFieldsFilled == 0 {
			parts := strings.Split(line, "-")
			if len(parts) > 1 {
				values := parseCommaSeparatedInts(parts[1])
				if len(values) > 0 {
					copy(currentData.RowRandomIndex[:], values)
					dataFieldsFilled++
				}
			}
		} else if strings.HasPrefix(line, "totalWin-") && dataFieldsFilled == 1 {
			parts := strings.Split(line, "-")
			if len(parts) > 1 {
				totalWin, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err == nil {
					currentData.CurrentTotalWin = totalWin
					dataFieldsFilled++
				}
			}
		} else if strings.HasPrefix(line, "gameMultiple-") && dataFieldsFilled == 2 {
			parts := strings.Split(line, "-")
			if len(parts) > 1 {
				gameMultiple, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err == nil {
					currentData.gameMultiple = gameMultiple
					dataFieldsFilled++
				}
			}
		} else if strings.HasPrefix(line, "stepMultiplier-") && dataFieldsFilled == 3 {
			parts := strings.Split(line, "-")
			if len(parts) > 1 {
				stepMultiplier, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err == nil {
					newData := *currentData
					newData.stepMultiplier = stepMultiplier
					newData.IsFirstTimes = isFirstTimes
					dataSet = append(dataSet, newData)
				}
			}
			isFirstTimes = false
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("文件读取错误: %v", err)
	}

	return dataSet, nil
}

func parseCommaSeparatedInts(s string) []int {
	parts := strings.Split(s, ",")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil {
			result = append(result, value)
		}
	}
	return result
}

// PrintGameTestDataSet 打印 GameTestData 切片
func PrintGameTestDataSet(dataSet GameTestDataSet) {
	fmt.Println("\n解析结果:")
	fmt.Printf("总共解析到 %d 条游戏数据\n\n", len(dataSet))

	for i, data := range dataSet {
		fmt.Printf("=== 游戏数据 #%d ===\n", i)
		fmt.Printf("RowRandomIndex:   %v\n", data.RowRandomIndex)
		fmt.Printf("CurrentTotalWin:  %d\n\n", data.CurrentTotalWin)
	}
}

// LoadAndPrintImportData 加载并打印导入的数据
func LoadAndPrintImportData() error {
	dataSet, err := ParseImportData()
	if err != nil {
		return err
	}
	PrintGameTestDataSet(dataSet)
	return nil
}

// PrintInfo RTP 调试输出
func PrintInfo(runtime int64, rtpIndex GameTestData, s *betOrderService, totalWin int64) {
	fmt.Printf("初始索引-%d,%d,%d,%d,%d\n", rtpIndex.RowRandomIndex[0], rtpIndex.RowRandomIndex[1],
		rtpIndex.RowRandomIndex[2], rtpIndex.RowRandomIndex[3], rtpIndex.RowRandomIndex[4])
	fmt.Printf("totalWin-%d--\n", rtpIndex.CurrentTotalWin)
	fmt.Printf("gameMultiple-%d--\n", rtpIndex.gameMultiple)
	fmt.Printf("stepMultiplier-%d--\n", rtpIndex.stepMultiplier)

	fmt.Printf("第几把--%d---------\n", runtime+1)
	for i := 0; i < _rowCount; i++ {
		fmt.Printf("cards11-%4d %4d %4d %4d %4d\n", s.symbolGrid[i][0], s.symbolGrid[i][1],
			s.symbolGrid[i][2], s.symbolGrid[i][3], s.symbolGrid[i][4])
	}
	fmt.Printf("totalWin-%d--\n", totalWin)
	fmt.Printf("gameMultiple-%d--\n", s.gameMultiple)
	fmt.Printf("stepMultiplier-%d--\n", s.stepMultiplier)
}

func saveToTXT(runtime int64, gameMultiple, stepMultiplier, totalWin int64, IsFreeRound bool, freeTimes uint64) error {
	file, err := os.OpenFile("测试.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	content := ""
	if IsFreeRound {
		content = fmt.Sprintf("基础模式第%d-免费第%d局\n", runtime+1, freeTimes)
	} else {
		content = fmt.Sprintf("基础模式第%d\n", runtime+1)
	}

	content += fmt.Sprintf("初始索引-%d,%d,%d,%d,%d\n", GameTestDataInfo.RowRandomIndex[0],
		GameTestDataInfo.RowRandomIndex[1], GameTestDataInfo.RowRandomIndex[2], GameTestDataInfo.RowRandomIndex[3],
		GameTestDataInfo.RowRandomIndex[4])
	content += fmt.Sprintf("totalWin-%d\n", totalWin)
	content += fmt.Sprintf("gameMultiple-%d\n", gameMultiple)
	content += fmt.Sprintf("stepMultiplier-%d\n", stepMultiplier)

	_, err = file.WriteString(content)
	return err
}

func saveToTXT1(runtime int64, gameMultiple, stepMultiplier, totalWin int64, IsFreeRound bool, freeTimes uint64, s *betOrderService) {
	file, err := os.OpenFile("测试全.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("无法打开日志文件: %v\n", err)
		return
	}
	defer file.Close()

	if IsFreeRound {
		fmt.Fprintf(file, "基础模式第%d局-免费第%d局\n", runtime+1, freeTimes)
	} else {
		fmt.Fprintf(file, "基础模式第%d局\n", runtime+1)
	}

	fmt.Fprintf(file, "初始索引-%d,%d,%d,%d,%d\n", GameTestDataInfo.RowRandomIndex[0],
		GameTestDataInfo.RowRandomIndex[1], GameTestDataInfo.RowRandomIndex[2], GameTestDataInfo.RowRandomIndex[3],
		GameTestDataInfo.RowRandomIndex[4])
	for i := 0; i < _rowCount; i++ {
		fmt.Fprintf(file, "symbolGrid-%4d %4d %4d %4d %4d\n", s.symbolGrid[i][0], s.symbolGrid[i][1],
			s.symbolGrid[i][2], s.symbolGrid[i][3], s.symbolGrid[i][4])
	}
	fmt.Fprintf(file, "中奖-%s\n", "----")
	for i := 0; i < _rowCount; i++ {
		fmt.Fprintf(file, "winGrid-%4d %4d %4d %4d %4d\n", s.winGrid[i][0], s.winGrid[i][1],
			s.winGrid[i][2], s.winGrid[i][3], s.winGrid[i][4])
	}
	fmt.Fprintf(file, "移动-%s\n", "----")
	moveSymbolGrid := s.moveSymbols()
	for i := 0; i < _rowCount; i++ {
		fmt.Fprintf(file, "moveSym-%4d %4d %4d %4d %4d\n", moveSymbolGrid[i][0], moveSymbolGrid[i][1],
			moveSymbolGrid[i][2], moveSymbolGrid[i][3], moveSymbolGrid[i][4])
	}
	fmt.Fprintf(file, "stepMultiplier-%d\n", stepMultiplier)
	fmt.Fprintf(file, "gameMultiple-%d\n", gameMultiple)
	fmt.Fprintf(file, "totalWin-%d\n", totalWin)
	if len(s.winInfos) > 0 {
		fmt.Fprintf(file, "WinResults:\n")
		for i, row := range s.winInfos {
			fmt.Fprintf(file, "  [%d]: {\n", i)
			fmt.Fprintf(file, "    \"symbol\": %d,\n", row.Symbol)
			fmt.Fprintf(file, "    \"symbolCount\": %d,\n", row.SymbolCount)
			fmt.Fprintf(file, "    \"lineCount\": %d,\n", row.LineCount)
			fmt.Fprintf(file, "    \"baseLineMultiplier\": %d,\n", row.Odds)
			fmt.Fprintf(file, "    \"totalMultiplier\": %d,\n", row.Multiplier)
			fmt.Fprintf(file, "  }")
			if i < len(s.winInfos)-1 {
				fmt.Fprintf(file, ",")
			}
			fmt.Fprintf(file, "\n")
		}
	} else {
		fmt.Fprintf(file, "WinResults: []\n")
	}
	fmt.Fprintf(file, "%s\n", "======================================")
}
