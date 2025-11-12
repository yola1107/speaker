package xslm2

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	mathRand "math/rand"
	"strconv"
	"strings"
	"sync"
)

// 随机数生成器
var randPool = &sync.Pool{
	New: func() any {
		var seed int64
		_ = binary.Read(rand.Reader, binary.LittleEndian, &seed)
		return mathRand.New(mathRand.NewSource(seed))
	},
}

// contains 检查值是否在切片中
func contains[T comparable](slice []T, val T) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// gridToString 网格转字符串（通用函数）
func gridToString(grid *int64Grid) string {
	if grid == nil {
		return ""
	}

	var b strings.Builder
	for r := int64(0); r < _rowCount; r++ {
		b.WriteString("[")
		for c := int64(0); c < _colCount; c++ {
			b.WriteString(strconv.FormatInt(grid[r][c], 10))
			if c < _colCount-1 {
				b.WriteString(",")
			}
		}
		b.WriteString("]")
		if r < _rowCount-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// hasWildSymbol 判断符号网格中是否有Wild符号
func hasWildSymbol(grid *int64Grid) bool {
	if grid == nil {
		return false
	}
	for _, row := range grid {
		for _, symbol := range row {
			if symbol == _wild {
				return true
			}
		}
	}
	return false
}

// getTreasureCount 获取符号网格中的夺宝符号数量
func getTreasureCount(grid *int64Grid) int64 {
	return countSymbol(grid, _treasure)
}

// countSymbol 统计符号网格中指定符号的数量（通用函数）
func countSymbol(grid *int64Grid, targetSymbol int64) int64 {
	count := int64(0)
	for _, row := range grid {
		for _, symbol := range row {
			// 跳过墙格标记
			if symbol == _blocked {
				continue
			}
			if symbol == targetSymbol {
				count++
			}
		}
	}
	return count
}

/*
// mergeWinGrids 合并多个中奖网格到目标网格
func mergeWinGrids(target *int64Grid, sources []int64Grid) {
	for _, source := range sources {
		for r := int64(0); r < _rowCount; r++ {
			for c := int64(0); c < _colCount; c++ {
				// 跳过墙格标记
				if source[r][c] == _blocked {
					continue
				}
				if source[r][c] != _blank {
					target[r][c] = source[r][c]
				}
			}
		}
	}
}

// isFemaleSymbol 判断是否为女性符号（7-9）
func isFemaleSymbol(symbol int64) bool {
	return symbol >= _femaleA && symbol <= _femaleC
}

// isFemaleWinSymbol 判断是否为触发女性连消的符号（含女性百搭）
func isFemaleWinSymbol(symbol int64) bool {
	return (symbol >= _femaleA && symbol <= _femaleC) || (symbol >= _wildFemaleA && symbol <= _wildFemaleC)
}

// isWildSymbol 判断是否为Wild相关符号（10-13）
func isWildSymbol(symbol int64) bool {
	return symbol >= _wildFemaleA && symbol <= _wild
}

// getFemaleCountsKey 生成配置key（"000"~"111"）
func getFemaleCountsKey(counts [3]int64) string {
	var result [3]byte
	for i := 0; i < 3; i++ {
		if counts[i] >= _femaleSymbolCountForFullElimination {
			result[i] = '1'
		} else {
			result[i] = '0'
		}
	}
	return string(result[:])
}
*/

// ToJSON json string
func ToJSON(v any) string {
	j, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

/*
🎯 消除规则对比表

模式             | 触发条件                      | 消除范围                     | 保护规则                          | 结算规则
----------------|--------------------------------|------------------------------|-----------------------------------|----------------
基础模式         | hasFemaleWin && hasWild        | 女性符号(7–9)、百搭(13)      | treasure 不消；含 treasure 的百搭保留 | 结算所有中奖符号
免费模式-全屏    | enableFullElimination = true   | 女性符号(7–9)、女性百搭(10–12) | treasure、百搭保留                 | 结算所有中奖符号
免费模式-部分    | hasFemaleWin = true            | 女性符号(7–9)、女性百搭(10–12) | treasure、百搭保留                 | 结算所有中奖符号
免费模式-其他    | 不满足上述条件                 | 不消除                        | -                                 | 结算所有中奖符号
*/

/*
每次 betOrder 只做一次消除，消除后立即下落和填充，保存处理后的网格供下次使用。即使还有消除，也要等下次调用。
betOrder->initSpin->findWinInfos-> 消除（根据winResult单次消除）->  有符号中奖消除时，滚轮上方的符号掉落补位-> 保存后续网格（即便还有消除）-> 下次betOrder
*/

/*

1.在普通模式中，出现百搭符号，且有女性符号中奖时，中奖的女性符号和百搭会消失，自然下落补齐。 如果下落补齐后的结果依然后中奖且中奖符号包含女性符号，那么女性符号继续消除，继续补齐，直到没有中奖。如果此种情况发生时屏幕上有夺宝符号，则百搭符号不消失，直到此次spin结束。
2.在免费游戏中，中奖的女性符号会消除（无需有百搭在场），自然下落补齐，如果下落补齐后的结果有中奖且中奖符号包含女性符号，那么女性符号继续消除，继续补齐，直到没有中奖。如果此种情况发生时屏幕上有百搭符号，则百搭符号不消息，直到次次spin结束
3.在免费游戏中，3中女性符号全部都可以转变为女性百搭后，有女性百搭参与的中奖符号都会消失（女性百搭符号会消失，但百搭符号不消失），自然下落补齐，意思是此时能有消除的玩法。
4.女性符号变女性百搭符号：女性符号共有3种，在免费游戏中，任一种女性符号中奖个数达到10，则该女性符号出现后转变为对应的女性百搭符号。
5.变消除玩法：在免费游戏中，3种女性符号全都可以转变为女性百搭后，有女性百搭符号参与的中奖符号（无论是不是女性符号）都会消失（女性百搭符号会消失，但百搭符号不消失），
6.新增免费次数： 基础模式： 获得3/4/5个夺宝时 赢得7/10/15次（配置文件读取）； 免费模式，每收集一个夺宝免费次数+1

*/

/*
   "1, free_spin_count对应scatter1~5个时，奖励的免费游戏数量",
    "free的滚轴用法：",
    "   记录ABC三个女性的收集数量，满十为1，否则为0；三个01拼接成key, 如000表示都未收集齐，010表示B收集齐，AC未收集齐",
    "   free期间每次计算都需要根据上文的key找滚轴数据，然后计算停轴结果及赔付登后续",
    "!!!!!!!!!!!!!!!!特别注意：",
    "由于本游戏中二级百搭(三个女性百搭)会位于第一列，故不同于普通waygame计算方法，这里需要考虑到所有情况，主要特殊情形包括：",
    "   1, 女性百搭居首时，非第一列甚至连续百搭之后的那一列都会中奖",
    "   2, 女性百搭不能相互替换，但是百搭可以替换女性百搭, 因此普通中奖和女性百搭中奖需要分开考虑"

*/

/*
	免费模式等key比例不对 平均一轮的免费游戏次数也不对；感觉是消除逻辑有问题或者收集ABC个数有问题 比如，一次游戏
	开始前三个收集是a,b,c
	中间消除三次
		第一次收集了x个a
		第二次收集了y个b
		第三次收集了z个c
	要这次游戏彻底结束（isRoundOver） 收集数量才从a,b,c更新到a+x, b+y, c+z
*/
