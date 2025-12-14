package champ3

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/looplab/fsm"
)

// ChampState 锦标赛状态
type ChampState int32

const (
	StateIdle       ChampState = iota // 初始状态（未开始）
	StateQualifying                   // 海选赛
	StateKnockout64                   // 淘汰赛64进32
	StateKnockout32                   // 淘汰赛32进16
	StateKnockout16                   // 淘汰赛16进8
	StateFinal                        // 决赛桌
	StateFinished                     // 锦标赛结束
)

// 转换为字符串表示
func (s ChampState) String() string {
	return [...]string{
		"idle",
		"qualifying",
		"knockout64",
		"knockout32",
		"knockout16",
		"final",
		"finished",
	}[s]
}

// PhaseInfo 阶段详细信息
type PhaseInfo struct {
	Name        string
	State       ChampState
	StartTime   time.Time     // 计划开始时间
	EndTime     time.Time     // 计划结束时间
	ActualStart time.Time     // 实际开始时间
	ActualEnd   time.Time     // 实际结束时间
	Duration    time.Duration // 实际持续时间
}

// ScheduleConfig 锦标赛时间表配置
type ScheduleConfig struct {
	QualifyingStart time.Time // 海选赛开始时间
	QualifyingEnd   time.Time // 海选赛结束时间
	Knockout64Start time.Time // 64强开始时间
	Knockout64End   time.Time // 64强结束时间
	Knockout32Start time.Time // 32强开始时间
	Knockout32End   time.Time // 32强结束时间
	Knockout16Start time.Time // 16强开始时间
	Knockout16End   time.Time // 16强结束时间
	FinalStart      time.Time // 决赛开始时间
	FinalEnd        time.Time // 决赛结束时间
}

// Championship 锦标赛对象
type Championship struct {
	ID           string
	Name         string
	CurrentState string                // 当前状态（字符串形式）
	phaseInfos   map[string]*PhaseInfo // 各阶段详细信息
	FSM          *fsm.FSM
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	scheduler    *time.Ticker // 调度器
	autoAdvance  bool         // 是否自动推进
	debug        bool         // 调试模式
}

// NewChampionship 创建锦标赛
func NewChampionship(id, name string, config *ScheduleConfig) *Championship {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Championship{
		ID:           id,
		Name:         name,
		CurrentState: StateIdle.String(),
		phaseInfos:   make(map[string]*PhaseInfo),
		ctx:          ctx,
		cancel:       cancel,
		autoAdvance:  true,
		debug:        false,
	}

	// 初始化阶段信息
	c.phaseInfos = map[string]*PhaseInfo{
		"idle": {
			Name:  "等待开始",
			State: StateIdle,
		},
		"qualifying": {
			Name:      "海选赛",
			State:     StateQualifying,
			StartTime: config.QualifyingStart,
			EndTime:   config.QualifyingEnd,
		},
		"knockout64": {
			Name:      "64强淘汰赛",
			State:     StateKnockout64,
			StartTime: config.Knockout64Start,
			EndTime:   config.Knockout64End,
		},
		"knockout32": {
			Name:      "32强淘汰赛",
			State:     StateKnockout32,
			StartTime: config.Knockout32Start,
			EndTime:   config.Knockout32End,
		},
		"knockout16": {
			Name:      "16强淘汰赛",
			State:     StateKnockout16,
			StartTime: config.Knockout16Start,
			EndTime:   config.Knockout16End,
		},
		"final": {
			Name:      "决赛桌",
			State:     StateFinal,
			StartTime: config.FinalStart,
			EndTime:   config.FinalEnd,
		},
		"finished": {
			Name:  "已结束",
			State: StateFinished,
		},
	}

	// 创建状态机
	c.FSM = fsm.NewFSM(
		c.CurrentState,
		fsm.Events{
			{Name: "start_qualifying", Src: []string{"idle"}, Dst: "qualifying"},
			{Name: "to_knockout64", Src: []string{"qualifying"}, Dst: "knockout64"},
			{Name: "to_knockout32", Src: []string{"knockout64"}, Dst: "knockout32"},
			{Name: "to_knockout16", Src: []string{"knockout32"}, Dst: "knockout16"},
			{Name: "to_final", Src: []string{"knockout16"}, Dst: "final"},
			{Name: "finish", Src: []string{"final"}, Dst: "finished"},
			{Name: "cancel", Src: []string{"idle", "qualifying"}, Dst: "finished"},
		},
		fsm.Callbacks{
			"enter_state": func(_ context.Context, e *fsm.Event) {
				c.enterState(e)
			},
			"leave_state": func(_ context.Context, e *fsm.Event) {
				c.leaveState(e)
			},
			// 特定状态回调
			"enter_qualifying": func(_ context.Context, e *fsm.Event) {
				fmt.Printf("=== 锦标赛 [%s] 海选赛开始！===\n", c.Name)
			},
			"enter_final": func(_ context.Context, e *fsm.Event) {
				fmt.Printf("=== 锦标赛 [%s] 决赛桌开始！===\n", c.Name)
			},
			"enter_finished": func(_ context.Context, e *fsm.Event) {
				fmt.Printf("=== 锦标赛 [%s] 已结束！===\n", c.Name)
			},
		},
	)

	// 启动调度器
	c.startScheduler()

	return c
}

// SetDebug 设置调试模式
func (c *Championship) SetDebug(debug bool) {
	c.debug = debug
}

// startScheduler 启动调度器
func (c *Championship) startScheduler() {
	// 使用更短的检查间隔
	c.scheduler = time.NewTicker(1000 * time.Millisecond)

	go func() {
		for {
			select {
			case <-c.ctx.Done():
				c.scheduler.Stop()
				if c.debug {
					fmt.Printf("[调度器] 停止\n")
				}
				return
			case <-c.scheduler.C:
				if c.autoAdvance {
					c.checkAndAdvance()
				}
			}
		}
	}()
}

// checkAndAdvance 检查并自动推进状态（修复死锁版本）
func (c *Championship) checkAndAdvance() {
	c.mu.Lock()

	now := time.Now()
	currentState := c.CurrentState
	var eventToTrigger string

	if c.debug {
		fmt.Printf("[调度器] 检查时间: %v, 当前状态: %s\n",
			now.Format("15:04:05.000"), currentState)
	}

	// 检查状态转换条件
	switch currentState {
	case "idle":
		if !now.Before(c.phaseInfos["qualifying"].StartTime) {
			eventToTrigger = "start_qualifying"
			if c.debug {
				fmt.Printf("[调度器] 满足条件: idle -> qualifying\n")
			}
		}
	case "qualifying":
		// 海选赛结束后，需要等待到64强开始时间
		if !now.Before(c.phaseInfos["qualifying"].EndTime) &&
			!now.Before(c.phaseInfos["knockout64"].StartTime) {
			eventToTrigger = "to_knockout64"
			if c.debug {
				fmt.Printf("[调度器] 满足条件: qualifying -> knockout64\n")
			}
		}
	case "knockout64":
		if !now.Before(c.phaseInfos["knockout64"].EndTime) &&
			!now.Before(c.phaseInfos["knockout32"].StartTime) {
			eventToTrigger = "to_knockout32"
			if c.debug {
				fmt.Printf("[调度器] 满足条件: knockout64 -> knockout32\n")
			}
		}
	case "knockout32":
		if !now.Before(c.phaseInfos["knockout32"].EndTime) &&
			!now.Before(c.phaseInfos["knockout16"].StartTime) {
			eventToTrigger = "to_knockout16"
			if c.debug {
				fmt.Printf("[调度器] 满足条件: knockout32 -> knockout16\n")
			}
		}
	case "knockout16":
		if !now.Before(c.phaseInfos["knockout16"].EndTime) &&
			!now.Before(c.phaseInfos["final"].StartTime) {
			eventToTrigger = "to_final"
			if c.debug {
				fmt.Printf("[调度器] 满足条件: knockout16 -> final\n")
			}
		}
	case "final":
		if !now.Before(c.phaseInfos["final"].EndTime) {
			eventToTrigger = "finish"
			if c.debug {
				fmt.Printf("[调度器] 满足条件: final -> finished\n")
			}
		}
	}

	c.mu.Unlock() // 关键：先释放锁，再触发事件

	// 在锁外触发事件，避免死锁
	if eventToTrigger != "" {
		if c.debug {
			fmt.Printf("[调度器] 触发事件: %s\n", eventToTrigger)
		}
		if err := c.FSM.Event(context.Background(), eventToTrigger); err != nil {
			fmt.Printf("触发事件 %s 失败: %v\n", eventToTrigger, err)
		}
	}
}

// enterState 进入状态的回调
func (c *Championship) enterState(e *fsm.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := e.Dst
	c.CurrentState = state

	if phaseInfo, exists := c.phaseInfos[state]; exists {
		phaseInfo.ActualStart = time.Now()
		fmt.Printf("[%s] 进入 %s 状态，实际开始时间: %v (计划: %v)\n",
			c.Name, phaseInfo.Name,
			phaseInfo.ActualStart.Format("2006-01-02 15:04:05"),
			phaseInfo.StartTime.Format("2006-01-02 15:04:05"),
		)
	}
}

// leaveState 离开状态的回调
func (c *Championship) leaveState(e *fsm.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := e.Src

	if phaseInfo, exists := c.phaseInfos[state]; exists {
		phaseInfo.ActualEnd = time.Now()
		if !phaseInfo.ActualStart.IsZero() {
			phaseInfo.Duration = phaseInfo.ActualEnd.Sub(phaseInfo.ActualStart)
		}
		fmt.Printf("[%s] 离开 %s 状态，实际结束时间: %v (持续: %v)\n",
			c.Name, phaseInfo.Name,
			phaseInfo.ActualEnd.Format("2006-01-02 15:04:05"),
			phaseInfo.Duration.Round(time.Millisecond))
	}
}

// GetCurrentPhase 获取当前阶段信息
func (c *Championship) GetCurrentPhase() *PhaseInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.phaseInfos[c.CurrentState]
}

// GetPhaseByTime 根据时间判断应该处于哪个阶段
func (c *Championship) GetPhaseByTime(t time.Time) (*PhaseInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查每个阶段的时间范围
	for state, info := range c.phaseInfos {
		if state == "idle" || state == "finished" {
			continue
		}

		// 如果时间在阶段的时间范围内
		if (info.StartTime.IsZero() || !t.Before(info.StartTime)) &&
			(info.EndTime.IsZero() || !t.After(info.EndTime)) {
			return info, nil
		}
	}

	// 检查是否在阶段之间
	return nil, fmt.Errorf("时间 %v 不在任何计划阶段内", t)
}

// IsInPhase 判断给定时间是否在某个阶段内
func (c *Championship) IsInPhase(state string, t time.Time) bool {
	info, exists := c.phaseInfos[state]
	if !exists {
		return false
	}

	return (info.StartTime.IsZero() || !t.Before(info.StartTime)) &&
		(info.EndTime.IsZero() || !t.After(info.EndTime))
}

// ManuallyAdvance 手动推进到下一阶段
func (c *Championship) ManuallyAdvance() error {
	var event string

	// 先确定要触发的事件（持有读锁）
	c.mu.RLock()
	switch c.CurrentState {
	case "idle":
		event = "start_qualifying"
	case "qualifying":
		event = "to_knockout64"
	case "knockout64":
		event = "to_knockout32"
	case "knockout32":
		event = "to_knockout16"
	case "knockout16":
		event = "to_final"
	case "final":
		event = "finish"
	default:
		c.mu.RUnlock()
		return fmt.Errorf("当前状态 %s 无法手动推进", c.CurrentState)
	}
	c.mu.RUnlock()

	// 在锁外触发事件
	return c.FSM.Event(context.Background(), event)
}

// SetAutoAdvance 设置是否自动推进
func (c *Championship) SetAutoAdvance(enabled bool) {
	c.autoAdvance = enabled
}

// Stop 停止锦标赛调度器
func (c *Championship) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// PrintSchedule 打印赛程表
func (c *Championship) PrintSchedule() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fmt.Printf("\n=== 锦标赛 [%s] 赛程表 ===\n", c.Name)

	states := []string{"qualifying", "knockout64", "knockout32", "knockout16", "final"}
	for _, state := range states {
		info := c.phaseInfos[state]
		status := "未开始"
		if !info.ActualStart.IsZero() {
			if info.ActualEnd.IsZero() {
				status = "进行中"
			} else {
				status = "已结束"
			}
		}

		fmt.Printf("%-15s: %s - %s (%s)\n",
			info.Name,
			info.StartTime.Format("01-02 15:04:05"),
			info.EndTime.Format("01-02 15:04:05"),
			status)
	}

	fmt.Printf("当前状态: %s\n", c.phaseInfos[c.CurrentState].Name)
}

// PrintDetailedStatus 打印详细状态
func (c *Championship) PrintDetailedStatus() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fmt.Printf("\n=== 锦标赛 [%s] 详细状态 ===\n", c.Name)
	fmt.Printf("当前阶段: %s\n", c.phaseInfos[c.CurrentState].Name)

	fmt.Println("\n各阶段详情:")
	states := []string{"qualifying", "knockout64", "knockout32", "knockout16", "final", "finished"}
	for _, state := range states {
		info := c.phaseInfos[state]
		fmt.Printf("  %s:\n", info.Name)
		if !info.StartTime.IsZero() {
			fmt.Printf("    计划开始: %s\n", info.StartTime.Format("2006-01-02 15:04:05"))
		}
		if !info.EndTime.IsZero() {
			fmt.Printf("    计划结束: %s\n", info.EndTime.Format("2006-01-02 15:04:05"))
		}
		if !info.ActualStart.IsZero() {
			fmt.Printf("    实际开始: %s\n", info.ActualStart.Format("2006-01-02 15:04:05"))
		}
		if !info.ActualEnd.IsZero() {
			fmt.Printf("    实际结束: %s\n", info.ActualEnd.Format("2006-01-02 15:04:05"))
			fmt.Printf("    持续时间: %v\n", info.Duration.Round(time.Millisecond))
		}
	}
}

// TestTimeBasedChampionship 测试时间驱动锦标赛（快速测试版）
func TestTimeBasedChampionship(t *testing.T) {
	fmt.Println("=== 测试时间驱动锦标赛（快速测试版）===")

	// 获取当前时间
	now := time.Now()

	// 设置较短的时间间隔用于快速测试
	config := &ScheduleConfig{
		QualifyingStart: now.Add(2 * time.Second), // 2秒后开始
		QualifyingEnd:   now.Add(5 * time.Second), // 5秒后结束

		Knockout64Start: now.Add(18 * time.Second), // 8秒后开始（间隔3秒）
		Knockout64End:   now.Add(22 * time.Second), // 12秒后结束

		Knockout32Start: now.Add(25 * time.Second), // 15秒后开始（间隔3秒）
		Knockout32End:   now.Add(28 * time.Second), // 18秒后结束

		Knockout16Start: now.Add(31 * time.Second), // 21秒后开始（间隔3秒）
		Knockout16End:   now.Add(34 * time.Second), // 24秒后结束

		FinalStart: now.Add(37 * time.Second), // 27秒后开始（间隔3秒）
		FinalEnd:   now.Add(40 * time.Second), // 30秒后结束
	}

	// 创建锦标赛
	champ := NewChampionship("time-champ-001", "时间驱动测试赛", config)
	champ.SetDebug(true) // 开启调试模式
	defer champ.Stop()

	fmt.Printf("测试开始时间: %v\n", now.Format("15:04:05.000"))
	champ.PrintSchedule()

	// 实时监控
	fmt.Println("\n=== 实时监控状态变化 ===")
	monitorTicker := time.NewTicker(1 * time.Second)
	defer monitorTicker.Stop()

	// 运行30秒
	timeout := time.After(35 * time.Second)

	for {
		select {
		case <-monitorTicker.C:
			current := champ.GetCurrentPhase()
			fmt.Printf("[%s] 当前阶段: %s\n",
				time.Now().Format("15:04:05"), current.Name)

		case <-timeout:
			fmt.Println("\n=== 测试结束 ===")
			champ.PrintDetailedStatus()
			return
		}
	}
}

// TestNonContinuousSchedule 测试非连续时间表的锦标赛
func TestNonContinuousSchedule(t *testing.T) {
	fmt.Println("=== 测试非连续时间表的锦标赛 ===")

	// 获取当前时间
	now := time.Now()

	// 设置非连续的时间表
	// 海选赛：明天 7-9点
	qualifyingDay := time.Date(now.Year(), now.Month(), now.Day()+1, 7, 0, 0, 0, now.Location())
	// 64强赛：后天 12-19点
	knockout64Day := time.Date(now.Year(), now.Month(), now.Day()+2, 12, 0, 0, 0, now.Location())
	// 32强赛：大后天 14-16点
	knockout32Day := time.Date(now.Year(), now.Month(), now.Day()+3, 14, 0, 0, 0, now.Location())
	// 16强赛：大后天 19-21点
	knockout16Day := time.Date(now.Year(), now.Month(), now.Day()+3, 19, 0, 0, 0, now.Location())
	// 决赛：大大后天 15-18点
	finalDay := time.Date(now.Year(), now.Month(), now.Day()+4, 15, 0, 0, 0, now.Location())

	config := &ScheduleConfig{
		QualifyingStart: qualifyingDay,
		QualifyingEnd:   qualifyingDay.Add(2 * time.Hour),

		Knockout64Start: knockout64Day,
		Knockout64End:   knockout64Day.Add(7 * time.Hour),

		Knockout32Start: knockout32Day,
		Knockout32End:   knockout32Day.Add(2 * time.Hour),

		Knockout16Start: knockout16Day,
		Knockout16End:   knockout16Day.Add(2 * time.Hour),

		FinalStart: finalDay,
		FinalEnd:   finalDay.Add(3 * time.Hour),
	}

	// 创建锦标赛
	champ := NewChampionship("test-champ-001", "非连续时间测试赛", config)
	defer champ.Stop()

	fmt.Printf("测试开始时间: %v\n", now.Format("2006-01-02 15:04:05"))
	champ.PrintSchedule()

	// 测试手动推进（跳过等待时间）
	fmt.Println("\n=== 手动推进测试 ===")

	statesToAdvance := []string{
		"开始海选赛",
		"进入64强赛",
		"进入32强赛",
		"进入16强赛",
		"进入决赛",
		"结束锦标赛",
	}

	for i, action := range statesToAdvance {
		fmt.Printf("%d. %s\n", i+1, action)
		if err := champ.ManuallyAdvance(); err != nil {
			fmt.Printf("手动推进失败: %v\n", err)
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	// 打印最终状态
	fmt.Println("\n=== 最终状态 ===")
	champ.PrintDetailedStatus()
}

// TestManualControl 测试手动控制
func TestManualControl(t *testing.T) {
	fmt.Println("=== 测试手动控制 ===")

	now := time.Now()

	config := &ScheduleConfig{
		QualifyingStart: now.Add(1 * time.Hour),
		QualifyingEnd:   now.Add(2 * time.Hour),
		Knockout64Start: now.Add(3 * time.Hour),
		Knockout64End:   now.Add(4 * time.Hour),
		Knockout32Start: now.Add(5 * time.Hour),
		Knockout32End:   now.Add(6 * time.Hour),
		Knockout16Start: now.Add(7 * time.Hour),
		Knockout16End:   now.Add(8 * time.Hour),
		FinalStart:      now.Add(9 * time.Hour),
		FinalEnd:        now.Add(10 * time.Hour),
	}

	champ := NewChampionship("manual-001", "手动控制测试赛", config)
	defer champ.Stop()

	// 关闭自动推进
	champ.SetAutoAdvance(false)

	fmt.Println("初始状态:")
	champ.PrintSchedule()

	// 手动逐步推进所有状态
	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		if err := champ.ManuallyAdvance(); err != nil {
			fmt.Printf("第%d次手动推进失败: %v\n", i+1, err)
			break
		}
		fmt.Printf("手动推进成功，当前状态: %s\n", champ.GetCurrentPhase().Name)
	}

	fmt.Println("\n最终状态:")
	champ.PrintDetailedStatus()
}

// TestGetPhaseByTime 测试根据时间判断阶段
func TestGetPhaseByTime(t *testing.T) {
	fmt.Println("=== 测试根据时间判断阶段 ===")

	now := time.Now()

	config := &ScheduleConfig{
		QualifyingStart: now.Add(1 * time.Hour),
		QualifyingEnd:   now.Add(3 * time.Hour),
		Knockout64Start: now.Add(4 * time.Hour),
		Knockout64End:   now.Add(6 * time.Hour),
		Knockout32Start: now.Add(7 * time.Hour),
		Knockout32End:   now.Add(9 * time.Hour),
		Knockout16Start: now.Add(10 * time.Hour),
		Knockout16End:   now.Add(12 * time.Hour),
		FinalStart:      now.Add(13 * time.Hour),
		FinalEnd:        now.Add(15 * time.Hour),
	}

	champ := NewChampionship("phase-test-001", "阶段测试赛", config)
	defer champ.Stop()

	// 测试不同时间点应该在哪个阶段
	testTimes := []struct {
		timeOffset time.Duration
		desc       string
	}{
		{30 * time.Minute, "海选赛开始前30分钟"},
		{1 * time.Hour, "海选赛进行中"},
		{3 * time.Hour, "海选赛结束，64强开始前"},
		{5 * time.Hour, "64强进行中"},
		{8 * time.Hour, "32强进行中"},
		{11 * time.Hour, "16强进行中"},
		{14 * time.Hour, "决赛进行中"},
		{16 * time.Hour, "所有比赛结束后"},
	}

	for _, tt := range testTimes {
		testTime := now.Add(tt.timeOffset)
		phase, err := champ.GetPhaseByTime(testTime)

		fmt.Printf("%s (时间: %v): ", tt.desc, testTime.Format("15:04:05"))
		if err != nil {
			fmt.Printf("错误 - %v\n", err)
		} else {
			fmt.Printf("应该在阶段 - %s\n", phase.Name)
		}
	}
}

// 运行所有测试
func TestAll(t *testing.T) {
	t.Run("时间驱动测试", TestTimeBasedChampionship)
	t.Run("非连续时间表测试", TestNonContinuousSchedule)
	t.Run("手动控制测试", TestManualControl)
	t.Run("时间点判断测试", TestGetPhaseByTime)
}
