package jqs

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

var debugLogging = false
var rtpTimes = int64(100000000)

var debugStartIndex = [][]int{}

var StepTranMap = map[int64]string{
	_spinTypeBase: "基础",
	_spinTypeFree: "免费",
}

func (s *betOrderService) printSymbolGrid(title string, symbolGrid int64Grid) {
	if !debugLogging {
		return
	}
	// 打印以千位数对齐
	s.PrintTips(title)
	for r := 0; r < _rowCount; r++ {
		for c := 0; c < _colCount; c++ {
			fmt.Printf("%4v ", symbolGrid[_rowCount-r-1][c])
		}
		fmt.Println()
	}

	s.PrintTips("")
}

func (s *betOrderService) PrintTips(title string) {
	if !debugLogging {
		return
	}
	totalWidth := 80
	titleWidth := utf8.RuneCountInString(title) * 2
	dashCount := (totalWidth - titleWidth) / 2
	if dashCount < 0 {
		dashCount = 0
	}
	dashes := strings.Repeat("-", dashCount)
	fmt.Println(fmt.Sprintf("%s%s%s", dashes, title, dashes))
}
