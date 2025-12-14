/*package champ2

import (
	"context"
	"fmt"
	"sync"
	"time"

	"hallSlot/common/protocol"

	log "github.com/sirupsen/logrus"
)

// ChampState 锦标赛状态（简化版：只保留3个主要状态）
type ChampState int32

const (
	StateIdle       ChampState = iota // 初始状态（未开始）
	StateQualifying                   // 海选赛
	StateKnockout                     // 淘汰赛（用RoundSize区分：64/32/16/8）
	StateFinal                        // 决赛桌
	StateFinished                     // 锦标赛结束
)

// String 返回状态名称
func (s ChampState) String() string {
	names := map[ChampState]string{
		StateIdle:       "Idle",
		StateQualifying: "Qualifying",
		StateKnockout:   "Knockout",
		StateFinal:      "Final",
		StateFinished:   "Finished",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}

// ToStageID 转换为阶段ID（int32）
func (s ChampState) ToStageID() int32 {
	switch s {
	case StateQualifying:
		return StageIDQualifying
	case StateKnockout:
		return StageIDKnockout
	case StateFinal:
		return StageIDFinal
	case StateIdle, StateFinished:
		return 0 // 初始状态和结束状态没有对应的阶段ID
	default:
		return 0
	}
}

// getNextRoundSize 获取淘汰赛下一轮的规模
func getNextRoundSize(currentSize int32) int32 {
	switch currentSize {
	case 64:
		return 32
	case 32:
		return 16
	case 16:
		return 8
	case 8:
		return 0 // 淘汰赛结束
	default:
		return 0
	}
}

const (
	qualifyingTotalRounds = 5 // 海选轮总共5局
	finalTotalRounds      = 5 // 决赛桌总共5轮
	playersPerTable       = 4 // 每桌人数
)

// ChampionshipStateMachine 锦标赛状态机（精简版）
type ChampionshipStateMachine struct {
	currentState ChampState
	champ        *Champ
	persistence  StatePersistence
	mu           sync.RWMutex

	// 回调函数（事件驱动：游戏结果触发）
	onStartGame   func(players []*Player, tables [][]int64, stageID int32, round int32, clientTableId int32) // 开始新游戏
	onFinishChamp func()                                                                                    // 锦标赛结束

	// 时间调度器（使用HeapScheduler，用于超时处理）
	scheduler Scheduler
}

// NewChampionshipStateMachine 创建状态机
func NewChampionshipStateMachine(champ *Champ, persistence StatePersistence) *ChampionshipStateMachine {
	// 创建HeapScheduler
	ctx := context.Background()
	scheduler := NewHeapScheduler(WithHeapContext(ctx), WithHeapWorkerPool(32, 1024))

	sm := &ChampionshipStateMachine{
		currentState: StateIdle,
		champ:        champ,
		persistence:  persistence,
		scheduler:    scheduler,
	}

	// 从持久化加载状态（根据RoundSize推断）
	if champ.RoundSize > 0 {
		sm.currentState = sm.stateFromRoundSize(champ.RoundSize)
	} else if len(champ.players) > 0 {
		// 如果有玩家但RoundSize为0，说明是海选赛
		sm.currentState = StateQualifying
	}

	return sm
}

// SetOnStartGame 设置游戏开始回调
func (sm *ChampionshipStateMachine) SetOnStartGame(callback func(players []*Player, tables [][]int64, stageID int32, round int32, clientTableId int32)) {
	sm.onStartGame = callback
}

// SetOnFinishChamp 设置锦标赛完成回调
func (sm *ChampionshipStateMachine) SetOnFinishChamp(callback func()) {
	sm.onFinishChamp = callback
}

// stateFromRoundSize 从RoundSize恢复状态
func (sm *ChampionshipStateMachine) stateFromRoundSize(roundSize int32) ChampState {
	if roundSize == 0 {
		// RoundSize为0可能是初始状态或海选赛
		// 如果有玩家，说明是海选赛；否则是初始状态
		if len(sm.champ.players) > 0 {
			return StateQualifying
		}
		return StateIdle
	}
	// 淘汰赛轮次：64, 32, 16, 8
	if roundSize >= 8 && roundSize <= 64 {
		return StateKnockout
	}
	// 决赛桌：RoundSize < 8 且 > 0
	if roundSize > 0 && roundSize < 8 {
		return StateFinal
	}
	return StateIdle
}

// GetCurrentState 获取当前状态
func (sm *ChampionshipStateMachine) GetCurrentState() ChampState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// TransitionTo 转换到目标状态
func (sm *ChampionshipStateMachine) TransitionTo(nextState ChampState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	oldState := sm.currentState
	if oldState == StateFinished {
		return nil
	}
	if nextState == StateFinished && oldState != StateFinal {
		log.Warnf("Invalid transition to Finished from %s", oldState.String())
		return nil
	}

	sm.currentState = nextState
	log.Infof("State transition: %s -> %s", oldState.String(), nextState.String())

	return sm.persistence.SaveState(sm.champ)
}

// StartQualifying 启动海选赛
func (sm *ChampionshipStateMachine) StartQualifying() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.currentState != StateIdle {
		return fmt.Errorf("can only start qualifying from Idle state, current state: %s", sm.currentState.String())
	}
	sm.currentState = StateQualifying
	log.Infof("Championship started: %s -> %s", StateIdle.String(), StateQualifying.String())
	return sm.persistence.SaveState(sm.champ)
}

// ProcessGameResult 处理游戏结果
func (sm *ChampionshipStateMachine) ProcessGameResult(result *protocol.ChampGameResult) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查阶段一致性（Idle状态时stage_id为0，允许通过）
	if sm.currentState != StateIdle && result.GetStageId() != sm.currentState.ToStageID() {
		log.Warnf("Stage mismatch: result stage_id=%d, current state=%s (stage_id=%d), ignoring",
			result.GetStageId(), sm.currentState.String(), sm.currentState.ToStageID())
		return nil
	}

	switch sm.currentState {
	case StateIdle:
		sm.currentState = StateQualifying
		sm.persistence.SaveState(sm.champ)
		return sm.handleQualifyingResult(result)
	case StateQualifying:
		return sm.handleQualifyingResult(result)
	case StateKnockout:
		return sm.handleKnockoutResult(result)
	case StateFinal:
		return sm.handleFinalResult(result)
	default:
		return nil
	}
}

// handleQualifyingResult 处理海选赛结果（result为nil表示超时触发）
func (sm *ChampionshipStateMachine) handleQualifyingResult(result *protocol.ChampGameResult) error {
	var activePlayers []*Player
	for _, p := range sm.champ.players {
		if p.EliminatedAt.IsZero() && p.Qualifying < qualifyingTotalRounds {
			activePlayers = append(activePlayers, p)
		}
	}

	if len(activePlayers) == 0 {
		return sm.advanceToKnockout()
	}

	sm.startQualifyingRound(activePlayers)
	return nil
}

// startQualifyingRound 启动海选轮（随机分组）
func (sm *ChampionshipStateMachine) startQualifyingRound(players []*Player) {
	playersByRound := make(map[int32][]*Player)
	for _, p := range players {
		playersByRound[p.Qualifying] = append(playersByRound[p.Qualifying], p)
	}

	for round, roundPlayers := range playersByRound {
		if len(roundPlayers) < playersPerTable {
			log.Debugf("Not enough players for qualifying round %d: %d", round+1, len(roundPlayers))
			continue
		}

		tables := RandomGrouping(roundPlayers, int32(len(roundPlayers)))
		if sm.onStartGame != nil {
			sm.onStartGame(roundPlayers, tables, StageIDQualifying, round+1, -1)
		}

		// 使用HeapScheduler安排超时任务（3分钟等待时间 + 游戏时间）
		// 超时后调用状态机的超时处理方法，统一处理逻辑
		roundNum := round + 1
		sm.ScheduleRoundTimeout(3*time.Minute+10*time.Minute, func() {
			sm.handleRoundTimeout(StageIDQualifying, roundNum)
		})

		log.Infof("Started qualifying round %d: players=%d, tables=%d", round+1, len(roundPlayers), len(tables))
	}
}

// advanceToKnockout 晋级到淘汰赛
func (sm *ChampionshipStateMachine) advanceToKnockout() error {
	sorted := SortPlayersByScore(sm.champ.GetActivePlayers())
	top64Count := 64
	if len(sorted) < top64Count {
		top64Count = len(sorted)
		log.Warnf("Less than 64 players for knockout: %d", top64Count)
	}
	top64 := sorted[:top64Count]

	// 淘汰未晋级的玩家
	top64UIDs := make(map[int64]bool, top64Count)
	for _, p := range top64 {
		top64UIDs[p.UID] = true
	}

	now := time.Now()
	for _, p := range sm.champ.players {
		if !top64UIDs[p.UID] {
			p.EliminatedAt = now
		}
	}

	sm.champ.InitRoundState(64)
	sm.currentState = StateKnockout
	if err := sm.persistence.SaveState(sm.champ); err != nil {
		return err
	}

	if sm.onStartGame != nil {
		tables := SnakeGrouping(top64, 64)
		sm.onStartGame(top64, tables, StageIDKnockout, 64, 0)
	}

	return nil
}

// handleKnockoutResult 处理淘汰赛结果（result为nil表示超时触发）
func (sm *ChampionshipStateMachine) handleKnockoutResult(result *protocol.ChampGameResult) error {
	if result != nil {
		// 检查这一局是否有玩家达到2胜（完成这个桌子）
		if !sm.isTableCompleted(result) {
			// 桌子未完成，继续等待
			return nil
		}
		// 桌子完成，标记为完成
		sm.champ.MarkTableCompleted(result.GetTableId())
	}

	// 检查是否所有桌子都完成了（必须等待所有桌子完成才能进入下一轮）
	if !sm.champ.IsRoundCompleted() {
		log.Debugf("Knockout round %d: waiting for all tables to complete. Completed: %d/%d",
			sm.champ.RoundSize, len(sm.champ.RoundTables), sm.champ.RoundSize/4)
		return nil
	}

	// 所有桌子都完成了，获取所有晋级玩家（WinCount >= 2）
	players := sm.champ.GetAdvancedPlayers()
	expectedPlayers := sm.champ.RoundSize / 2 // 每个桌子4人，只有1人晋级，所以是 RoundSize/2

	if len(players) < expectedPlayers {
		log.Warnf("Knockout round %d: expected %d advanced players, but got %d. Waiting...",
			sm.champ.RoundSize, expectedPlayers, len(players))
		return nil
	}

	// 如果玩家数量超过预期，只取前 expectedPlayers 个（按积分排序）
	if len(players) > expectedPlayers {
		log.Warnf("Knockout round %d: got %d advanced players, but expected %d. Taking top %d.",
			sm.champ.RoundSize, len(players), expectedPlayers, expectedPlayers)
		sorted := SortPlayersByScore(players)
		players = sorted[:expectedPlayers]
	}

	nextRoundSize := getNextRoundSize(sm.champ.RoundSize)
	if nextRoundSize == 0 {
		// 淘汰赛结束（8强），进入决赛桌
		sm.currentState = StateFinal
		sm.champ.InitRoundState(1)
		if err := sm.persistence.SaveState(sm.champ); err != nil {
			return err
		}

		if sm.onStartGame != nil {
			if len(players) >= 8 {
				tables := FinalRoundGrouping(players, 1)
				sm.onStartGame(players, tables, StageIDFinal, 1, 0)
			}
		}
		return nil
	}

	// 进入下一轮淘汰赛
	sm.champ.InitRoundState(nextRoundSize)
	if err := sm.persistence.SaveState(sm.champ); err != nil {
		return err
	}

	if sm.onStartGame != nil {
		tables := SnakeGrouping(players, nextRoundSize)
		sm.onStartGame(players, tables, StageIDKnockout, nextRoundSize, 0)
	}

	log.Infof("Knockout round %d completed, advanced to round %d with %d players",
		sm.champ.RoundSize, nextRoundSize, len(players))
	return nil
}

// isTableCompleted 判断桌位是否完成（淘汰赛：有玩家达到2胜）
// 注意：这个方法在 updatePlayerResult 之后调用，此时 WinCount 已经更新
func (sm *ChampionshipStateMachine) isTableCompleted(result *protocol.ChampGameResult) bool {
	now := time.Now()

	// 检查这一局是否有玩家达到2胜（完成这个桌子）
	for _, pr := range result.GetPlayers() {
		p, exists := sm.champ.players[pr.GetUid()]
		if !exists {
			continue
		}

		// 如果这一局玩家获胜，WinCount 已经在 updatePlayerResult 中更新
		// 所以这里检查的是更新后的 WinCount
		if pr.GetIsWinner() && p.WinCount >= 2 {
			// 找到获胜者（达到2胜），淘汰同桌其他玩家
			for _, loser := range result.GetPlayers() {
				if loser.GetUid() != p.UID {
					if lp, ok := sm.champ.players[loser.GetUid()]; ok && lp.EliminatedAt.IsZero() {
						lp.EliminatedAt = now
						log.Debugf("Player %d eliminated in table %d (winner: %d reached 2 wins)",
							loser.GetUid(), result.GetTableId(), p.UID)
					}
				}
			}
			log.Infof("Table %d completed: player %d reached 2 wins and advanced",
				result.GetTableId(), p.UID)
			return true
		}
	}

	// 没有玩家达到2胜，桌子未完成，继续下一局
	return false
}

// handleFinalResult 处理决赛桌结果（result为nil表示超时触发）
func (sm *ChampionshipStateMachine) handleFinalResult(result *protocol.ChampGameResult) error {
	if result != nil {
		sm.champ.MarkTableCompleted(result.GetTableId())
		if !sm.champ.IsRoundCompleted() {
			return nil
		}
	} else {
		// 超时触发：检查是否所有桌子完成
		if !sm.champ.IsRoundCompleted() {
			return nil
		}
	}

	if sm.champ.RoundSize >= finalTotalRounds {
		sm.currentState = StateFinished
		if err := sm.persistence.SaveState(sm.champ); err != nil {
			return err
		}

		if sm.onFinishChamp != nil {
			sm.onFinishChamp()
		}
		return nil
	}

	nextRound := sm.champ.RoundSize + 1
	sm.champ.InitRoundState(nextRound)
	if err := sm.persistence.SaveState(sm.champ); err != nil {
		return err
	}

	if sm.onStartGame != nil {
		players := sm.champ.GetActivePlayers()
		if len(players) < 8 {
			log.Warnf("Not enough players for final round: %d", len(players))
			return nil
		}

		tables := FinalRoundGrouping(players, nextRound)
		sm.onStartGame(players, tables, StageIDFinal, nextRound, 0)
	}

	return nil
}

// StartScheduler 启动时间调度器（使用HeapScheduler）
func (sm *ChampionshipStateMachine) StartScheduler() error {
	// HeapScheduler在创建时已经启动，这里可以安排初始任务
	return nil
}

// StopScheduler 停止时间调度器
func (sm *ChampionshipStateMachine) StopScheduler() {
	if sm.scheduler != nil {
		sm.scheduler.Stop()
	}
}

// ScheduleRoundTimeout 安排轮次超时任务（3分钟等待时间）
func (sm *ChampionshipStateMachine) ScheduleRoundTimeout(duration time.Duration, callback func()) int64 {
	if sm.scheduler == nil {
		return -1
	}
	return sm.scheduler.Once(duration, callback)
}

// handleRoundTimeout 处理轮次超时（统一超时处理逻辑）
func (sm *ChampionshipStateMachine) handleRoundTimeout(stageID int32, round int32) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Warnf("Round timeout: stage=%d, round=%d", stageID, round)

	switch stageID {
	case StageIDQualifying:
		// 海选轮超时：标记未完成的桌子，淘汰未进入的玩家
		sm.handleQualifyingTimeout()
		sm.mu.Unlock()
		sm.handleQualifyingResult(nil) // 传入nil表示超时触发
		sm.mu.Lock()

	case StageIDKnockout:
		// 淘汰赛超时：标记未完成的桌子，淘汰未进入的玩家
		// 注意：淘汰赛必须等待所有桌子完成（3局2胜），超时后强制完成未完成的桌子
		sm.handleKnockoutTimeout()
		sm.mu.Unlock()
		sm.handleKnockoutResult(nil)
		sm.mu.Lock()

	case StageIDFinal:
		// 决赛桌超时：标记未完成的桌子
		sm.handleFinalTimeout()
		sm.mu.Unlock()
		sm.handleFinalResult(nil)
		sm.mu.Lock()
	}
}

// handleKnockoutTimeout 处理淘汰赛超时
func (sm *ChampionshipStateMachine) handleKnockoutTimeout() {
	now := time.Now()
	totalTables := int(sm.champ.RoundSize / 4)
	completedTables := len(sm.champ.RoundTables)

	if completedTables < totalTables {
		log.Warnf("Knockout timeout: %d/%d tables completed, forcing completion",
			completedTables, totalTables)

		// 对于未完成的桌子，需要选择玩家晋级
		// 策略：选择所有活跃玩家中 WinCount 最高的玩家晋级
		activePlayers := sm.champ.GetActivePlayers()
		expectedPlayers := sm.champ.RoundSize / 2

		if len(activePlayers) > expectedPlayers {
			// 按 WinCount 降序排序
			sorted := SortPlayersByScore(activePlayers) // 这个方法也会考虑 WinCount
			// 选择前 expectedPlayers 个玩家晋级，淘汰其他玩家
			for i := expectedPlayers; i < len(sorted); i++ {
				if sorted[i].EliminatedAt.IsZero() {
					sorted[i].EliminatedAt = now
					log.Debugf("Player %d eliminated due to timeout (WinCount=%d)",
						sorted[i].UID, sorted[i].WinCount)
				}
			}
		}

		// 标记所有桌子为完成（强制完成）
		// 注意：这里无法知道具体的 tableId，所以通过确保玩家数量正确来间接完成
		// 实际上，IsRoundCompleted 会检查完成的桌子数量，这里我们通过确保玩家数量来满足条件
	}
}

// handleQualifyingTimeout 处理海选轮超时
func (sm *ChampionshipStateMachine) handleQualifyingTimeout() {
	// 标记未完成的桌子为完成
	for tableId, completed := range sm.champ.RoundTables {
		if !completed {
			log.Warnf("Qualifying table %d timeout, marking as completed", tableId)
			sm.champ.MarkTableCompleted(tableId)
		}
	}
}

// handleFinalTimeout 处理决赛桌超时
func (sm *ChampionshipStateMachine) handleFinalTimeout() {
	// 标记未完成的桌子为完成
	for tableId, completed := range sm.champ.RoundTables {
		if !completed {
			log.Warnf("Final table %d timeout, marking as completed", tableId)
			sm.champ.MarkTableCompleted(tableId)
		}
	}
}
*/

package champ2
