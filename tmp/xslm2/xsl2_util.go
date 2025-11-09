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
			// 跳过墙格标记
			if symbol == _blocked {
				continue
			}
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
总结对比表
模式	                 消除条件	            消除范围	                         保护规则
基础模式  	  hasFemaleWin && hasWild	   只消除女性符号(7,8,9)和wild(13)	    treasure不消除；wild有treasure时不消除
免费模式-全屏	  enableFullElimination=true   消除所有中奖符号	                treasure和wild不消除
免费模式-部分	  hasFemaleWin=true	           只消除女性符号(7,8,9)	            treasure和wild不消除
免费模式-其他	  都不满足	                     不消除	                         -
*/

/*
每次 betOrder 只做一次消除，消除后立即下落和填充，保存处理后的网格供下次使用。即使还有消除，也要等下次调用。
betOrder->initSpinSymbol->findWinInfos-> 消除（根据winResult单次消除）->  有符号中奖消除时，滚轮上方的符号掉落补位-> 保存后续网格（即便还有消除）-> 下次betOrder
*/

/*

1. Ways 与符号替换
规则：女性百搭只替换对应女性符号，普通百搭可替任何符号；女性百搭在第一列时依旧能把后续列拉进中奖。
代码：xsl2_spin.go -> findNormalSymbolWinInfo 里新增的 isMatchingFemaleWild 正是按这个规则来做的；普通百搭 (_wild) 依然通用，女性百搭会被严格匹配到 7/8/9 自己，符合文档。

2. 连消与消除
基础模式：女性中奖 + Wild 才触发连消；有夺宝不消 Wild。
processStepForBase：只有 hasFemaleWin && hasWildSymbol 时才 updateStepResults(true)，同时 finalizeStep 里对 _wild 做了保护，和规则一致。
免费模式：仅女性中奖触发连消，全屏模式时不看女性也延续，夺宝不消，百搭保留与否取决于夺宝。
processStepForFree 按“全屏优先、女性中奖其次、否则结算”来写，finalizeStep 里对 _treasure / _wild 的处理也符合“夺宝留在盘面、普通百搭可消”。

3. 女性收集与百搭演化
规则：连消后统计 7/8/9，满 10 转成 10/11/12，免费局保留状态。
代码：collectFemaleSymbols、checkFullElimination、initSpinSymbol 会依据 femaleCountsForFree 选择免费滚轴并转成百搭；场景保存 (SpinSceneData.FemaleCountsForFree) 也保证跨请求持久化，符合规则。

4. 免费次数赠送
规则：基础局 3/4/5 夺宝送 7/10/15 次；免费局每出一个夺宝 +1；文档强调“连消结束时统计”且夺宝留在原位。
代码现状：目前 if isFreeRound { newFreeRoundCount = treasureCount - roundStartTreasure }，只考量“盘面差值”。如果连消中新增的夺宝在结算时不在盘面，就不会被统计到——这就是你看到 RTP 掉到 0.66 的原因。
建议：改成“本回合累计出现多少个夺宝就加多少”，即在 finalizeStep 或 collectFemaleSymbols 阶段维护计数，而不是看最终盘面。这个偏差说明代码与文档在“夺宝统计方式”上还没完全对齐，需要进一步修正。

5. 场景/调试相关
SpinSceneData 持久化了女性计数、下一步网格、滚轴 key 等；调试输出也能看到女性/夺宝、滚轴信息。与文档描述的“免费保留状态”“滚轴依收集调整”是一致的。
总结：核心玩法（Ways、消除、百搭演化）都已按 doc1/doc2 实现，只有“免费局夺宝送次数”的统计方式还没完全对应。建议把这个部分改成“按回合内出现的夺宝累计 +1”，并继续用 rtp_test 验证，这样才能兼顾文档要求与 RTP 结果。其它部分已和文档对照无差。

*/

/*

在普通模式中，出现百搭符号，且有女性符号中奖时，中奖的女性符号和百搭会消失，自然下落补齐。 如果下落补齐后的结果依然后中奖且中奖符号包含女性符号，那么女性符号继续消除，继续补齐，直到没有中奖。如果此种情况发生时屏幕上有夺宝符号，则百搭符号不消失，直到此次spin结束。
在免费游戏中，中奖的女性符号会消除（无需有百搭在场），自然下落补齐，如果下落补齐后的结果有中奖且中奖符号包含女性符号，那么女性符号继续消除，继续补齐，直到没有中奖。如果此种情况发生时屏幕上有百搭符号，则百搭符号不消息，直到次次spin结束
在免费游戏中，3中女性符号全部都可以转变为女性百搭后，有女性百搭参与的中奖符号（无论是不是女性符号）都会消失（女性百搭符号会消失，但百搭符号不消失），自然下落补齐，意思是此时能有消除的玩法。

新增免费次数： 基础模式： 获得3/4/5个夺宝时 赢得7/10/15次（配置文件读取）； 免费模式，每收集一个夺宝免费次数+1

*/
