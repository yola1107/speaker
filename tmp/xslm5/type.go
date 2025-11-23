package xslm3

import (
	"fmt"
	"strings"
)

type int64Grid = [_rowCount][_colCount]int64

type winInfo struct {
	Symbol      int64
	SymbolCount int64
	LineCount   int64
	WinGrid     int64Grid
}

type winResult struct {
	Symbol             int64     `json:"symbol"`
	SymbolCount        int64     `json:"symbolCount"`
	LineCount          int64     `json:"lineCount"`
	BaseLineMultiplier int64     `json:"baseLineMultiplier"`
	TotalMultiplier    int64     `json:"totalMultiplier"`
	WinGrid            int64Grid `json:"winGrid"`
}

type rtpDebugData struct {
	open bool // 是否开启调试模式
}

/*
初始状态: _spinTypeBase (1)
    |
    | [有消除]
    v
_spinTypeBaseEli (11) -> [继续消除] -> _spinTypeBaseEli (11)
    |
    | [无消除，有免费次数]
    v
_spinTypeFree (21) -> [有消除] -> _spinTypeFreeEli (22) -> [继续消除] -> _spinTypeFreeEli (22)
    |                                                              |
    | [无消除，还有免费次数]                                        | [无消除，免费次数用完]
    +--------------------------------------------------------------+
    |
    v
_spinTypeBase (1) [重新开始]
*/

/*
	只有每次新的免费游戏发起  才会改变key  从而改变滚轴、女性转变以及消除逻辑


检查循环消除逻辑：fallingWinSymbols已更新SymbolRoller，下一轮handleSymbolGrid会从更新后的SymbolRoller读取

游戏规则：
1）游戏为3x4x4x4x3的way game老虎机。
2）中奖不消除，单回合一把结束，但有特殊情况，见下描述。
3）
4）普通模式中，出现百搭符号，且有女性符号中奖时，中奖的女性符号和百搭符号会消失，自然下落补齐。如果下落补齐后的结果依然有中奖且中奖符号包含女性符号，那么女性符号继续消除，继续补齐，直到没有中奖。如果此种情况发生时屏幕上有夺宝符号，则百搭符号不消失，直到此次spin结束。
5）在免费游戏中，中奖的女性符号会消除（无需有百搭符号在场），自然下落补齐，如果下落补齐后的结果有中奖且中奖符号包含女性符号，那么女性符号继续消除，继续补齐，直到没有中奖。如果此种情况发生时屏幕上有百搭符号，则百搭符号不消失，直到此次spin结束；
6).普通模式收集3个夺宝符号赢得7次免费游戏，4个10次，5个15次。在免费游戏中每收集1个夺宝符号则免费游戏次数+1
7.)女性符号共有3种，在免费游戏中，任一种女性符号中奖个数达到10，则该女性符号出现后转变为对应的女性百搭符号
8.)在免费游戏中，3种女性符号全都可以转变为女性百搭后，有女性百搭符号参与的中奖符号（无论是不是女性符号）都会消失（女性百搭符号会消失，但百搭符号不消失），自然下落补齐
9).只有每次新的免费游戏发起  才会改变key  从而改变滚轴、女性转变以及消除逻辑; 新游戏可以用（isRoundOver标识）
10）消除规则：
	基础模式：消除中奖的女性符号（7，8，9）及百搭，如果盘面有夺宝则百搭不消除
	免费模式：
		1> 全屏情况：每个中奖Way找女性百搭，找到则改way除百搭13之外的符号都全部消除
		2> 非全屏情况：每个中奖way找女性，找到该way女性及女性百搭都消除
*/

func printGrid(grid *int64Grid, winGrid *int64Grid) string {
	buf := &strings.Builder{}
	if grid == nil {
		buf.WriteString("(空)\n")
		return ""
	}
	rGrid := reverseGridRows(grid)
	rWinGrid := reverseGridRows(winGrid)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			symbol := rGrid[r][c]
			isWin := rWinGrid[r][c] != _blank && rWinGrid[r][c] != _blocked
			if isWin {
				if symbol == _blank {
					buf.WriteString("   *|")
				} else {
					fmt.Fprintf(buf, " %2d*|", symbol)
				}
			} else {
				if symbol == _blank {
					buf.WriteString("    |")
				} else {
					fmt.Fprintf(buf, " %2d |", symbol)
				}
			}
			if c < _colCount-1 {
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

// reverseGridRows 网格行序反转
func reverseGridRows(grid *int64Grid) int64Grid {
	if grid == nil {
		return int64Grid{}
	}
	var reversed int64Grid
	for i := int64(0); i < _rowCount; i++ {
		reversed[i] = grid[_rowCount-1-i]
	}
	return reversed
}
