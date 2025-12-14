package champ2

//// SnakeGrouping 蛇形分组（按积分排序后蛇形分配）
//func SnakeGrouping(players []*Player, roundSize int32) [][]int64 {
//	// 按积分降序排序
//	sorted := SortPlayersByScore(players)
//
//	// 计算桌数（4人一桌）
//	tableCount := int(roundSize / 4)
//	tables := make([][]int64, tableCount)
//	for i := range tables {
//		tables[i] = make([]int64, 0, 4)
//	}
//
//	// 蛇形填充：1,4,5,8 一桌；2,3,6,7 一桌
//	for i, player := range sorted {
//		tableIndex := i % tableCount
//		if (i/tableCount)%2 == 1 {
//			// 反向填充
//			tableIndex = tableCount - 1 - tableIndex
//		}
//		tables[tableIndex] = append(tables[tableIndex], player.UID)
//	}
//
//	return tables
//}

//// RandomGrouping 随机分组（用于海选轮和决赛桌第1轮）
//func RandomGrouping(players []*Player, roundSize int32) [][]int64 {
//	// 复制并打乱
//	shuffled := make([]*Player, len(players))
//	copy(shuffled, players)
//
//	// Fisher-Yates 洗牌算法
//	rand.Seed(time.Now().UnixNano())
//	for i := len(shuffled) - 1; i > 0; i-- {
//		j := rand.Intn(i + 1)
//		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
//	}
//
//	// 计算桌数
//	tableCount := int(roundSize / 4)
//	tables := make([][]int64, tableCount)
//	for i := range tables {
//		tables[i] = make([]int64, 0, 4)
//	}
//
//	// 顺序分配
//	for i, player := range shuffled {
//		tableIndex := i % tableCount
//		tables[tableIndex] = append(tables[tableIndex], player.UID)
//	}
//
//	return tables
//}

//// FinalRoundGrouping 决赛桌分组（第1轮随机，后续轮次蛇形）
//func FinalRoundGrouping(players []*Player, round int32) [][]int64 {
//	if round == 1 {
//		// 第1轮：随机分桌
//		return RandomGrouping(players, 8)
//	}
//
//	// 第2轮及以后：按积分蛇形分组
//	sorted := SortPlayersByScore(players)
//
//	// 8强分2桌：A桌(1,4,5,8)，B桌(2,3,6,7)
//	return [][]int64{
//		{sorted[0].UID, sorted[3].UID, sorted[4].UID, sorted[7].UID}, // A桌
//		{sorted[1].UID, sorted[2].UID, sorted[5].UID, sorted[6].UID}, // B桌
//	}
//}

//// SortPlayersByScore 按积分降序排序玩家
//func SortPlayersByScore(players []*Player) []*Player {
//	sorted := make([]*Player, len(players))
//	copy(sorted, players)
//
//	// 使用标准库排序
//	sort.Slice(sorted, func(i, j int) bool {
//		if sorted[i].Score != sorted[j].Score {
//			return sorted[i].Score > sorted[j].Score
//		}
//		// 积分相同，比较第一名次数
//		if sorted[i].FirstPlaceCount != sorted[j].FirstPlaceCount {
//			return sorted[i].FirstPlaceCount > sorted[j].FirstPlaceCount
//		}
//		// 第一名次数相同，比较第二名次数
//		return sorted[i].SecondPlaceCount > sorted[j].SecondPlaceCount
//	})
//
//	return sorted
//}
//
//// RankInfo 排名信息
//type RankInfo struct {
//	Rank        int32 // 排名
//	UID         int64 // 玩家ID
//	Score       int64 // 积分
//	WinCount    int32 // 胜场数（淘汰赛用）
//	FirstCount  int32 // 第一名次数
//	SecondCount int32 // 第二名次数
//}
//
//// GetRankings 获取排名列表（按积分排序）
//func GetRankings(champ *Champ, limit int) []*RankInfo {
//	players := champ.GetActivePlayers()
//	if len(players) == 0 {
//		return nil
//	}
//
//	// 排序
//	sorted := SortPlayersByScore(players)
//
//	rankings := make([]*RankInfo, 0, len(sorted))
//	for i, player := range sorted {
//		if limit > 0 && i >= limit {
//			break
//		}
//		rankings = append(rankings, &RankInfo{
//			Rank:        int32(i + 1),
//			UID:         player.UID,
//			Score:       player.Score,
//			WinCount:    player.WinCount,
//			FirstCount:  player.FirstPlaceCount,
//			SecondCount: player.SecondPlaceCount,
//		})
//	}
//
//	return rankings
//}
//
//// GetTop64Players 获取前64名玩家（用于海选轮晋级）
//func GetTop64Players(champ *Champ) []int64 {
//	rankings := GetRankings(champ, 64)
//	uids := make([]int64, 0, len(rankings))
//	for _, rank := range rankings {
//		uids = append(uids, rank.UID)
//	}
//	return uids
//}
