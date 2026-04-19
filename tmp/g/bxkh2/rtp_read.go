package bxkh2

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GameTestData 游戏测试数据结构体，用于批量处理测试数据
type GameTestData struct {

	// 生成矩阵阶段：realIndex数组
	realIndex [4]int
	// 生成矩阵阶段：randomIndex数组
	randomIndex [4]int
	// 生成矩阵阶段：RowRandomIndex数组
	RowRandomIndex [6]int
	// 当前累计中奖金额
	CurrentTotalWin int
	// 免费倍数
	freeMultiple int
	// 当前中奖金额
	stepMultiplier int
	// 是否为第一次spin（遇到stepMultiplier-0时为true）
	IsFirstTimes bool
}

// GameTestDataSet 游戏测试数据集切片类型
type GameTestDataSet []GameTestData

// GameTestDataInfo 全局测试数据实例，用于调试
var GameTestDataInfo GameTestData

// ParseImportData 解析importData.txt文件并生成GameTestData切片
func ParseImportData() (GameTestDataSet, error) {
	// 构建文件路径
	filePath := filepath.Join("/Users/q/Documents/UGit/egame-grpc03/game/bxkh/importData.txt")

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %v", err)
	}
	defer file.Close()

	// 初始化数据集
	dataSet := make(GameTestDataSet, 0)
	var currentData *GameTestData
	var dataFieldsFilled int
	// 标记是否为第一次遇到stepMultiplier-0
	isFirstTimes := true

	// 读取文件内容
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") || strings.Contains(line, "PASS") || strings.Contains(line, "Runtime") {
			continue // 跳过空行、注释行和统计行
		}

		// 检查是否为新spin标识 - 仅当严格等于"stepMultiplier-0"时才作为新spin开始
		if strings.HasPrefix(line, "基础模式第") {
			// 重置状态，准备新的spin数据
			currentData = nil
			dataFieldsFilled = 0
			// 设置为第一次
			isFirstTimes = true
		}

		// 如果当前没有活动的数据项，创建一个新的ß
		if currentData == nil {
			currentData = &GameTestData{}
			dataFieldsFilled = 0
			continue
		}

		// 解析数据行
		if strings.HasPrefix(line, "realIndex-") && dataFieldsFilled == 0 {
			// 解析RandomTailArray
			parts := strings.Split(line, "-")
			if len(parts) > 1 {
				values := parseCommaSeparatedInts(parts[1])
				// 填充RandomTailArray，前两个位置固定为0，然后填充解析的值
				if len(values) > 0 {
					copy(currentData.realIndex[:], values)
					dataFieldsFilled++
					//fmt.Printf("解析RandomTailArray: %v, 字段填充数: %d\n", currentData.RandomTailArray, dataFieldsFilled)
				}
			}
			// 判断行类型并解析数据
		} else if strings.HasPrefix(line, "randomIndex-") && dataFieldsFilled == 1 {
			// 解析RandomTailArray
			parts := strings.Split(line, "-")
			if len(parts) > 1 {
				values := parseCommaSeparatedInts(parts[1])
				// 填充RandomTailArray，前两个位置固定为0，然后填充解析的值
				if len(values) > 0 {
					copy(currentData.randomIndex[:], values)
					dataFieldsFilled++
					//fmt.Printf("解析RandomTailArray: %v, 字段填充数: %d\n", currentData.RandomTailArray, dataFieldsFilled)
				}
			}
			// 判断行类型并解析数据
		} else if strings.HasPrefix(line, "初始索引-") && dataFieldsFilled == 2 {
			// 解析RandomTailArray
			parts := strings.Split(line, "-")
			if len(parts) > 1 {
				values := parseCommaSeparatedInts(parts[1])
				// 填充RandomTailArray，前两个位置固定为0，然后填充解析的值
				if len(values) > 0 {
					copy(currentData.RowRandomIndex[:], values)
					dataFieldsFilled++
					//fmt.Printf("解析RandomTailArray: %v, 字段填充数: %d\n", currentData.RandomTailArray, dataFieldsFilled)
				}
			}
		} else if strings.HasPrefix(line, "totalWin-") && dataFieldsFilled == 3 { // 解析totalWin行
			if currentData != nil {
				parts := strings.Split(line, "-")
				if len(parts) > 1 {
					totalWin, err := strconv.Atoi(strings.TrimSpace(parts[1]))
					if err == nil {
						currentData.CurrentTotalWin = totalWin
						dataFieldsFilled++
						//fmt.Printf("解析totalWin并创建新实例: %d, 数据集长度: %d\n", newData.CurrentTotalWin, len(dataSet))
					}
				}
			}
		} else if strings.HasPrefix(line, "freeMultiple-") && dataFieldsFilled == 4 { // 解析freeMultiple行
			if currentData != nil {
				parts := strings.Split(line, "-")
				if len(parts) > 1 {
					freeMultiple, err := strconv.Atoi(strings.TrimSpace(parts[1]))
					if err == nil {
						currentData.freeMultiple = freeMultiple
						dataFieldsFilled++
						//fmt.Printf("解析totalWin并创建新实例: %d, 数据集长度: %d\n", newData.CurrentTotalWin, len(dataSet))
					}
				}
			}
		} else if strings.HasPrefix(line, "stepMultiplier-") && dataFieldsFilled == 5 { // 解析stepMultiplier行
			if currentData != nil {
				parts := strings.Split(line, "-")
				if len(parts) > 1 {
					stepMultiplier, err := strconv.Atoi(strings.TrimSpace(parts[1]))
					if err == nil {
						// 创建一个新的struct实例，复制当前数据
						newData := *currentData
						newData.stepMultiplier = stepMultiplier
						// 设置isFirstTimes标志
						newData.IsFirstTimes = isFirstTimes
						dataSet = append(dataSet, newData)
						//fmt.Printf("解析totalWin并创建新实例: %d, 数据集长度: %d\n", newData.CurrentTotalWin, len(dataSet))
					}
				}
			}
			// 处理完当前的tstepMultiplier后，设置isFirstTimes为false
			isFirstTimes = false
			continue
		}

		// 注：移除了自动添加数据的逻辑，现在只在遇到stepMultiplier-0或文件结束时添加数据
	}

	// 不再需要保存最后一组数据，因为totalWin行已经处理了数据的保存

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("文件读取错误: %v", err)
	}

	return dataSet, nil
}

// parseCommaSeparatedInts 解析逗号分隔的整数字符串
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

// PrintGameTestDataSet 打印GameTestData切片的内容
func PrintGameTestDataSet(dataSet GameTestDataSet) {
	fmt.Println("\n解析结果:")
	fmt.Printf("总共解析到 %d 条游戏数据\n\n", len(dataSet))

	for i, data := range dataSet {
		fmt.Printf("=== 游戏数据 #%d ===\n", i)
		fmt.Printf("RowRandomIndex:   %v\n", data.RowRandomIndex)
		fmt.Printf("CurrentTotalWin:  %d\n\n", data.CurrentTotalWin)
	}
}

// LoadAndPrintImportData 加载并打印导入的数据，用于测试解析结果
func LoadAndPrintImportData() error {
	dataSet, err := ParseImportData()
	if err != nil {
		return err
	}

	PrintGameTestDataSet(dataSet)
	return nil
}

func PrintInfo(rtpIndex GameTestData, res BaseSpinResult, totalWin int64) {
	fmt.Printf("初始索引-%d,%d,%d,%d,%d,%d\n", rtpIndex.RowRandomIndex[0], rtpIndex.RowRandomIndex[1],
		rtpIndex.RowRandomIndex[2], rtpIndex.RowRandomIndex[3], rtpIndex.RowRandomIndex[4],
		rtpIndex.RowRandomIndex[5])
	fmt.Printf("totalWin-%d--\n", rtpIndex.CurrentTotalWin)
	fmt.Printf("freeMultiple-%d--\n", rtpIndex.freeMultiple)
	fmt.Printf("stepMultiplier-%d--\n", rtpIndex.stepMultiplier)

	fmt.Printf("第几把----------------------\n")
	for i := 0; i < _rowCount; i++ {
		fmt.Printf("cards11-%4d %4d %4d %4d %4d %4d\n", res.cards[i][0], res.cards[i][1], res.cards[i][2], res.cards[i][3], res.cards[i][4], res.cards[i][5])
	}
	fmt.Printf("totalWin-%d--\n", totalWin)
	fmt.Printf("freeMultiple-%d--\n", res.freeMultiple)
	fmt.Printf("stepMultiplier-%d--\n", res.stepMultiplier)
}

func saveToTXT(runtime int64, freeMultiple, stepMultiplier, totalWin int64, IsFreeRound bool, freeTimes uint64) error {

	file, err := os.OpenFile("测试.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 构建要写入的内容
	content := fmt.Sprintf("普通第几把-----%d,免费第几把------%d\n", runtime+1, freeTimes)
	content += fmt.Sprintf("realIndex-%d,%d,%d,%d\n",
		GameTestDataInfo.realIndex[0], GameTestDataInfo.realIndex[1],
		GameTestDataInfo.realIndex[2], GameTestDataInfo.realIndex[3])
	content += fmt.Sprintf("randomIndex-%d,%d,%d,%d\n",
		GameTestDataInfo.randomIndex[0], GameTestDataInfo.randomIndex[1],
		GameTestDataInfo.randomIndex[2], GameTestDataInfo.randomIndex[3])
	content += fmt.Sprintf("初始索引-%d,%d,%d,%d,%d,%d\n", GameTestDataInfo.RowRandomIndex[0],
		GameTestDataInfo.RowRandomIndex[1], GameTestDataInfo.RowRandomIndex[2], GameTestDataInfo.RowRandomIndex[3],
		GameTestDataInfo.RowRandomIndex[4], GameTestDataInfo.RowRandomIndex[5])
	content += fmt.Sprintf("totalWin-%d\n", totalWin)
	content += fmt.Sprintf("freeMultiple-%d\n", freeMultiple)
	content += fmt.Sprintf("stepMultiplier-%d\n", stepMultiplier)

	_, err = file.WriteString(content)
	return err
}

func saveToTXT1(runtime int64, freeMultiple, stepMultiplier, totalWin int64, IsFreeRound bool, freeTimes uint64, res *BaseSpinResult) {

	file, err := os.OpenFile("测试全.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("无法打开日志文件: %v\n", err)
		return
	}
	defer file.Close()

	// 将你的打印语句改为写入文件
	fmt.Fprintf(file, "普通第几把-----%d,免费第几把------%d\n", runtime+1, freeTimes)
	fmt.Fprintf(file, "realIndex-%d,%d,%d,%d\n", GameTestDataInfo.realIndex[0], GameTestDataInfo.realIndex[1],
		GameTestDataInfo.realIndex[2], GameTestDataInfo.realIndex[3])
	fmt.Fprintf(file, "randomIndex-%d,%d,%d,%d\n", GameTestDataInfo.randomIndex[0], GameTestDataInfo.randomIndex[1],
		GameTestDataInfo.randomIndex[2], GameTestDataInfo.randomIndex[3])
	fmt.Fprintf(file, "初始索引-%d,%d,%d,%d,%d,%d\n", GameTestDataInfo.RowRandomIndex[0],
		GameTestDataInfo.RowRandomIndex[1], GameTestDataInfo.RowRandomIndex[2], GameTestDataInfo.RowRandomIndex[3],
		GameTestDataInfo.RowRandomIndex[4], GameTestDataInfo.RowRandomIndex[5])
	for i := 0; i < _rowCount; i++ {
		fmt.Fprintf(file, "cards-%4d %4d %4d %4d %4d %4d\n", res.cards[i][0], res.cards[i][1], res.cards[i][2], res.cards[i][3], res.cards[i][4], res.cards[i][5])
	}
	//fmt.Fprintf(file, "中奖-%s\n", "----")
	//for i := 0; i < _rowCount; i++ {
	//	fmt.Fprintf(file, "winGrid-%4d %4d %4d %4d %4d %4d\n", res.winGrid[i][0], res.winGrid[i][1], res.winGrid[i][2], res.winGrid[i][3], res.winGrid[i][4], res.winGrid[i][5])
	//}
	//fmt.Fprintf(file, "移动-%s\n", "----")
	//for i := 0; i < _rowCount; i++ {
	//	fmt.Fprintf(file, "moveSym-%4d %4d %4d %4d %4d %4d\n", res.moveSymbolGrid[i][0], res.moveSymbolGrid[i][1], res.moveSymbolGrid[i][2], res.moveSymbolGrid[i][3], res.moveSymbolGrid[i][4], res.moveSymbolGrid[i][5])
	//}
	fmt.Fprintf(file, "stepMultiplier-%d\n", stepMultiplier)
	fmt.Fprintf(file, "freeMultiple-%d\n", freeMultiple)
	fmt.Fprintf(file, "totalWin-%d\n", totalWin)

	// 添加分隔符，便于区分不同的游戏回合
	fmt.Fprintf(file, "%s\n", "======================================")

}
