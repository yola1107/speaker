package xxg2

/*

	Config
	[1,2,3,3]
	[3,9,5,4]
	[6,3,3,4]
	[8,9,3,4]
	[1,3,3,4]

	stepMap:
	[1,3,6,8,1,   2,9,3,9,3,   3,5,3,3,3,   3,4,4,4,4]

	symbolGrid:
	[1,3,6,8,1]
	[2,9,3,9,3]
	[3,5,3,3,3]
	[3,4,4,4,4]

	// 免费模式
	1>Spin 生成符号
	2>计算s的数量个数
	3>如果s数量大于等于3个，蝙蝠移动（生成新框，新框不能超过最大配置限制参数5个)
	4>判断新框是否转换Wind（老人小孩女人）
	5>算分中奖
	6>是否触发免费次数 （免费游戏中每个夺宝符号增加2次免费次数）
	7>下一轮 （剩余次数够？）

	//基础模式
	1>Spin 生成符号
	2>计算s的数量个数
	3>判断新框是否转换Wind（老人小孩女人）（两个及一个s的情况）
	4>算分中奖
	5>是否触发免费次数（三个s及以上）
	6>下一轮

*/

/*
蝙蝠数据流转说明：

1. stepMap.TreatPos/TreatCount（统一数据源）
   - loadStepData() 扫描本轮盘面，保存treasure(11号符号)位置和数量到stepMap
   - 整个spin流程从stepMap读取

2. scene.BatPositions（持久化状态）
   - 保存蝙蝠**移动后**的位置
   - 基础触发免费时：stepMap.TreatPos → scene.BatPositions
   - 免费每轮更新：从stepMap.Bat提取移动后位置 → scene.BatPositions
   - 持久化到Redis，整个免费游戏期间保持

3. stepMap.Bat（动画数据）
   - 记录蝙蝠完整移动信息：(X,Y)→(TransX,TransY)，Syb→Sybn
   - 用途：前端播放蝙蝠飞行/射线动画
   - 生命周期：单次spin

数据流转：
  基础模式(有3个treasure) → stepMap.TreatPos=[pos1,pos2,pos3]
                          → scene.BatPositions=[pos1,pos2,pos3]（保存）

  免费第1轮 → scene.BatPositions读取[pos1,pos2,pos3]
          → moveBatOneStep移动 → pos1',pos2',pos3'
          → stepMap.Bat=[{X:pos1,TransX:pos1',...},...]
          → scene.BatPositions=[pos1',pos2',pos3']（更新）

  免费第2轮 → scene.BatPositions读取[pos1',pos2',pos3']
          → 重复上述流程...
*/
