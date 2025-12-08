package hbtr2

// 以'行'为组
type int64Grid [_rowCount][_colCount]int64

// WinInfo 中奖元素
type WinInfo struct {
	Symbol      int64     `json:"val"`     // 符号值
	SymbolCount int64     `json:"starNum"` // 连续相同符号的个数
	LineCount   int64     `json:"roadNum"` // 路数（支付线编号，从0开始）
	Odds        int64     `json:"odds"`    // 基础赔率
	Multiplier  int64     `json:"mul"`     // 倍数
	WinGrid     int64Grid `json:"loc"`     // 中奖位置网格
}

// CardType 牌型信息
type CardType struct {
	Type     int // 牌型
	Way      int // 路数 0~24
	Multiple int // 倍数
	Route    int // 几路 中了记录，0，3，4，5
}

// position 位置坐标
type position struct {
	Row    int64 `json:"r"` // 行（0-5）
	Col    int64 `json:"c"` // 列（0-6）
	Symbol int64 `json:"s"` // 符号
}

// rtpDebugData RTP调试数据
type rtpDebugData struct {
	open bool  // 是否开启调试模式（用于RTP测试时的详细日志输出）
	mark int32 // 组合标记：基础模式0-99，免费模式100-199。低位表示状态：1=有wild,2=有wild移动,4=wild->scatter转换,8=有scatter
}

// Bat Wild移动记录，用于前端播放飞行动画
type Bat struct {
	X      int64 `json:"x"`    // 起始行（服务器）→ 起始列（客户端）
	Y      int64 `json:"y"`    // 起始列（服务器）→ 起始行（客户端）
	TransX int64 `json:"nx"`   // 目标行（服务器）→ 目标列（客户端）
	TransY int64 `json:"ny"`   // 目标列（服务器）→ 目标行（客户端）
	Syb    int64 `json:"syb"`  // 原符号
	Sybn   int64 `json:"sybn"` // 新符号（转换后）
}

/*
// 网格布局说明：
// 第0行：从realData[6]逆序取4个符号构成中间4列，两边填0
// 第1-4行：从realData[0-5]各取4个符号垂直填充所有列
// 总共5行×6列，索引0-29用于WinLines配置

服务器生成坐标
[
    [0,  2, 9, 3, 8, 0]             <-  realData[6] [fall , start]
    [6,  5, 6, 2, 8, 5],
    [8,  4,10, 3, 8, 7],
    [5,  2, 8, 9, 3, 5],
    [5, 10, 9, 4, 3, 9],
]

发送给前端的网格
[
    [5, 10, 9, 4, 3, 9],
    [5,  2, 8, 9, 3, 5],
    [8,  4,10, 3, 8, 7],
    [6,  5, 6, 2, 8, 5],
    [0,  2, 9, 3, 8, 0]
]


	### 网格布局（5行 × 6列）

	```
			 Col 0    Col 1    Col 2    Col 3    Col 4   Col 5
	Row 0:  [0][0]   [0][1]   [0][2]   [0][3]   [0][4]   [0][5]      (0, 1, 2, 3, 4, 5)
	Row 1:  [1][0]   [1][1]   [1][2]   [1][3]   [1][4]   [1][5]      (6, 7, 8, 9,10, 11)
	Row 2:  [2][0]   [2][1]   [2][2]   [2][3]   [2][4]   [2][5]      (12,13,14,15,16,17)
	Row 3:  [3][0]   [3][1]   [3][2]   [3][3]   [3][4]   [3][5]      (18,19,20,21,22,23)
	Row 4:  [4][0]   [4][1]   [4][2]   [4][3]   [4][4]   [4][5]      (24,25,26,27,28,29)
	```
	```
			 Col 0    Col 1    Col 2    Col 3    Col 4   Col 5
	Row 0:  [0]      [1]      [2]      [3]      [4]     [5]                   ←  对应 realData[fall,start] + 2个墙格
	Row 1:  [6]      [7]      [8]      [9]      [10]    [11]                  ←
	Row 2:  [12]     [13]     [14]     [15]     [16]    [17]                  ←
	Row 3:  [18]     [19]     [20]     [21]     [22]    [23]                  ←
	Row 4:  [24]     [25]     [26]     [27]     [28]    [29]                  ←
			↑
			一维索引值（在 WinLines 配置中使用）
	```


	最终：
	第 1 行从 realData[6]（逆序取 4）构成中间 4 列，两边填 0
	第 2～5 行按列从 realData[0]～realData[5] 各取 4 个符号垂直填充
*/

// moveSymbolsOriginal 原版实现，用于验证优化版本的正确性
func (s *betOrderService) moveSymbolsOriginal(grid *int64Grid) *int64Grid {
	/*
		处理第0行：水平左移动（对应roller下标[6]）
		逻辑：从左到右扫描，如果当前位置是空位，从右侧找到第一个非空非wild符号向左移动
		注意：[0][0] [0][5]是墙格符号为0，只处理中间4列（列1-4）
		示例：[0, 4, 0, 8] -> [4, 8, 0, 0]
	*/
	for c := int64(1); c < _colCount-1; c++ {
		if grid[0][c] != 0 {
			continue
		}
		// 如果当前位置是空位，从右侧找有效符号（非0且非wild）填充
		for k := c + 1; k < _colCount-1; k++ {
			if val := grid[0][k]; val != 0 && !isWild(val) {
				grid[0][c] = val
				grid[0][k] = 0
				break
			}
		}
	}

	/*
		处理第1-4行：垂直下落（对应roller下标[0-5]）
		逻辑：从下往上扫描每列，将非wild非0符号向下压缩到底部，wild位置保持不变
		示例：初始 [5, 0, 7, 0, 9] → 结果 [0, 0, 5, 7, 9]
	*/
	for col := int64(0); col < _colCount; col++ {
		// 初始化写入位置：从底部开始，如果是墙格列则跳过第0行
		writePos := int64(_rowCount - 1)
		if isBlockedCell(0, col) {
			writePos = _rowCount - 1 // 墙格列从第4行开始
		}

		// 从下往上扫描第1-4行，将非wild非0符号下落
		for row := int64(_rowCount - 1); row >= 1; row-- {
			// 如果writePos已经超出范围，提前退出
			if writePos < 1 {
				break
			}

			// 跳过墙格位置（墙格不会下落，保持为0）
			if isBlockedCell(row, col) {
				continue
			}

			val := grid[row][col]

			// 处理wild位置：wild位置保持不变，但writePos需要跳过这个位置
			if isWild(val) {
				// 如果wild在writePos上方，需要调整writePos跳过wild位置
				if row < writePos {
					writePos = row - 1
					// 确保writePos不会指向墙格
					for writePos >= 0 && isBlockedCell(writePos, col) {
						writePos--
					}
				}
				continue
			}

			// 处理非空非wild符号：向下移动到writePos位置
			if val != 0 {
				// 确保writePos不是墙格位置
				for writePos >= 0 && isBlockedCell(writePos, col) {
					writePos--
				}
				if writePos < 1 {
					break // 没有可写入位置，退出
				}

				// 如果当前位置和目标位置不同，执行移动
				if row != writePos {
					grid[writePos][col] = val
					grid[row][col] = 0
				}
				writePos-- // 下一个写入位置上移

				// 确保writePos不会指向墙格
				for writePos >= 0 && isBlockedCell(writePos, col) {
					writePos--
				}
			}
			// 空位（val == 0）直接跳过，不处理
		}
	}

	return grid
}
