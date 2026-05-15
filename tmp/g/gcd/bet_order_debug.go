package gcd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

const _ = "https://wjq224.axshare.com/#g=1&id=9zmum8&p=%E9%A1%B9%E7%9B%AE%E8%AF%B4%E6%98%8E_3"

var debugLogging = false

var rtpTimes = int64(100000000)

var _DebugJson []*DebugJson
var debugTimes = 0
var freeInFree = int64(0)

type DebugJson struct {
	Stop    []int `json:"stop"`
	Display []int `json:"display"`
	Win     int   `json:"scoreAllLine"`
}

func loadDebugConf() {
}

var stepTranMap = map[int64]string{
	_normalMode:    "基础",
	_normalModeEli: "基础(respin)",
	_freeMode:      "免费",
	_freeModeEli:   "免费(respin)",
}

func debugPrintSymbolGrid(title string, symbolGrid Int64Grid) {
	if !debugLogging {
		return
	}
	debugPrintTips(title)
	for r := int64(0); r < _rowCount; r++ {
		for c := int64(0); c < _colCount; c++ {
			commonPrint(fmt.Sprintf("%3v ", symbolGrid[_rowCount-r-1][c]))
		}
		commonPrint("\n")
	}
	debugPrintTips("")
}

func debugPrintTips(title string) {
	if !debugLogging {
		return
	}
	totalWidth := 100
	titleWidth := utf8.RuneCountInString(title) * 2
	dashCount := (totalWidth - titleWidth) / 2
	if dashCount < 0 {
		dashCount = 0
	}
	dashes := strings.Repeat("-", dashCount)
	commonPrint(fmt.Sprintf("%s%s%s", dashes, title, dashes))
	commonPrint("\n")
}

func debugPrintln(title ...string) {
	if len(title) > 0 {
		for _, s := range title {
			commonPrint(s)
			commonPrint("\n")
		}
	} else {
		commonPrint("\n")
	}
}

var (
	debugBuffer     strings.Builder
	debugBufferMu   sync.Mutex
	debugBufferOpen bool = false
)

func openDebugBuffer() {
	debugBufferOpen = true
}

func commonPrint(s string) {
	fmt.Print(s)
	if debugBufferOpen {
		debugBufferMu.Lock()
		defer debugBufferMu.Unlock()
		debugBuffer.WriteString(s)
	}
}

func writeDebugBufferToFile(filename string) error {
	if !debugBufferOpen {
		return nil
	}
	debugBufferMu.Lock()
	defer debugBufferMu.Unlock()

	content := debugBuffer.String()
	if err := os.MkdirAll("log", 0755); err != nil {
		return fmt.Errorf("创建 log 目录失败: %w", err)
	}
	file, err := os.OpenFile(fmt.Sprintf("logs/%s", filename), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

func clearDebugBuffer() {
	if !debugBufferOpen {
		return
	}
	debugBufferMu.Lock()
	defer debugBufferMu.Unlock()
	debugBuffer.Reset()
}

func writeDebugBufferToFileAndClear(filename string) {
	if !debugBufferOpen {
		return
	}
	if err := writeDebugBufferToFile(filename); err != nil {
		panic(err)
	}
	clearDebugBuffer()
}
