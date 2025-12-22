package champv2

import (
	"container/heap"
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	//log "github.com/sirupsen/logrus"
)

/*import (
	"context"
	"time"
)

type DelayScheduler struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewDelayScheduler() *DelayScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &DelayScheduler{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Schedule 延迟执行任务
func (s *DelayScheduler) Schedule(t time.Time, fn func()) {
	delay := time.Until(t)
	if delay <= 0 {
		// 如果时间已经过去，直接执行
		go fn()
		return
	}

	go func() {
		select {
		case <-time.After(delay):
			fn()
		case <-s.ctx.Done():
			return
		}
	}()
}

func (s *DelayScheduler) Stop() {
	s.cancel()
}
*/

// Scheduler 定时任务调度器接口
type Scheduler interface {
	Once(delay time.Duration, f func()) int64          // 注册一次性任务（延迟执行）
	ScheduleAt(execTime time.Time, f func()) int64     // 注册一次性任务（指定时间执行）
	Forever(interval time.Duration, f func()) int64    // 注册周期任务
	ForeverNow(interval time.Duration, f func()) int64 // 注册周期任务并立即执行一次
	Cancel(taskID int64)                               // 取消指定任务
	CancelAll()                                        // 取消所有任务
	Stop()                                             // 停止调度器
}

// ==================== heapTaskEntry ====================

// heapTaskEntry 堆调度器任务项
type heapTaskEntry struct {
	id        int64         // 任务ID
	execAt    time.Time     // 下一次执行时间
	interval  time.Duration // 周期任务间隔
	repeated  bool          // 是否周期任务
	cancelled atomic.Bool   // 是否已取消
	task      func()        // 任务函数
	index     int           // 堆索引，用于从堆中删除
}

// taskQueue 任务队列，使用最小堆管理定时任务
type taskQueue struct {
	mu    sync.Mutex               // 保护并发访问
	heap  []*heapTaskEntry         // 最小堆，按执行时间排序
	tasks map[int64]*heapTaskEntry // 任务ID到任务的映射，用于快速查找
}

func newTaskQueue() *taskQueue {
	return &taskQueue{
		heap:  make([]*heapTaskEntry, 0),
		tasks: make(map[int64]*heapTaskEntry),
	}
}

// 实现 heap.Interface
func (q *taskQueue) Len() int { return len(q.heap) }
func (q *taskQueue) Less(i, j int) bool {
	return q.heap[i].execAt.Before(q.heap[j].execAt)
}
func (q *taskQueue) Swap(i, j int) {
	q.heap[i], q.heap[j] = q.heap[j], q.heap[i]
	q.heap[i].index = i
	q.heap[j].index = j
}
func (q *taskQueue) Push(x any) {
	t := x.(*heapTaskEntry)
	t.index = len(q.heap)
	q.heap = append(q.heap, t)
}
func (q *taskQueue) Pop() any {
	n := len(q.heap)
	t := q.heap[n-1]
	t.index = -1
	q.heap = q.heap[:n-1]
	return t
}

func (q *taskQueue) AddTask(t *heapTaskEntry) bool {
	if t == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks[t.id] = t
	needWake := len(q.heap) == 0 || t.execAt.Before(q.heap[0].execAt)
	heap.Push(q, t)
	return needWake
}

func (q *taskQueue) PopExpired(now time.Time) []*heapTaskEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	var expired []*heapTaskEntry
	for len(q.heap) > 0 && !q.heap[0].execAt.After(now) {
		t := heap.Pop(q).(*heapTaskEntry)
		delete(q.tasks, t.id)
		if !t.cancelled.Load() {
			expired = append(expired, t)
		}
	}
	return expired
}

func (q *taskQueue) RemoveTask(taskID int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	t, ok := q.tasks[taskID]
	if !ok {
		return
	}
	// 只标记为取消，不立即清空 task，避免正在执行的任务 panic
	t.cancelled.Store(true)
	delete(q.tasks, taskID)
	if t.index >= 0 && t.index < len(q.heap) {
		heap.Remove(q, t.index)
	}
}

func (q *taskQueue) TaskCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

func (q *taskQueue) NextExecDuration(now time.Time) time.Duration {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.heap) == 0 {
		return time.Hour
	}
	if d := q.heap[0].execAt.Sub(now); d > 0 {
		return d
	}
	return 0
}

// ==================== HeapSchedulerOption ====================

type HeapSchedulerOption func(*heapScheduler)

// WithWorkerPool 任务池并发模式，size 个 worker，队列容量 queueSize
func WithWorkerPool(pool *WorkerPool) HeapSchedulerOption {
	return func(s *heapScheduler) { s.workerPool = pool }
}

// WithHeapSerial 任务池串行模式（一个 goroutine 顺序跑任务）
func WithHeapSerial() HeapSchedulerOption {
	return func(s *heapScheduler) { s.workerPool = NewWorkerPool(1, 1024) }
}

// WithHeapWorkerPool 任务池并发模式，size 个 worker，队列容量 queueSize
func WithHeapWorkerPool(size, queueSize int) HeapSchedulerOption {
	return func(s *heapScheduler) { s.workerPool = NewWorkerPool(size, queueSize) }
}

func WithHeapContext(ctx context.Context) HeapSchedulerOption {
	return func(s *heapScheduler) { s.ctx = ctx }
}

// heapScheduler 基于最小堆的定时任务调度器
type heapScheduler struct {
	queue      *taskQueue         // 任务队列
	nextID     atomic.Int64       // 任务ID生成器
	shutdown   atomic.Bool        // 调度器是否已关闭
	timer      *time.Timer        // 定时器，用于等待下一个任务
	ctx        context.Context    // 上下文，用于控制调度器生命周期
	cancel     context.CancelFunc // 取消函数
	wg         sync.WaitGroup     // 等待所有任务完成
	wakeup     chan struct{}      // 唤醒信号通道，用于新任务唤醒循环
	workerPool *WorkerPool        // 可选任务池
}

func NewHeapScheduler(opts ...HeapSchedulerOption) Scheduler {
	s := &heapScheduler{
		queue:  newTaskQueue(),
		wakeup: make(chan struct{}, 1),
		ctx:    context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.ctx, s.cancel = context.WithCancel(s.ctx)
	s.timer = time.NewTimer(time.Hour)
	s.timer.Stop() // 立即停止，等待首次 Reset
	go s.loop()
	return s
}

func (s *heapScheduler) loop() {
	defer RecoverFromError(func(e any) { go s.loop() })
	for {
		expired := s.queue.PopExpired(time.Now())
		for _, t := range expired {
			// 先保存任务函数，避免在异步执行时访问可能被修改的字段
			taskFn := t.task
			if taskFn == nil {
				continue
			}

			s.wg.Add(1)
			task := t
			submitTask := func() {
				defer func() {
					RecoverFromError(nil)
					s.wg.Done()
				}()
				// 执行前再次检查是否已取消
				if !task.cancelled.Load() {
					taskFn()
				}
			}

			// 执行任务
			if s.workerPool != nil {
				s.workerPool.Submit(submitTask)
			} else {
				go submitTask()
			}

			// 周期任务重新入队
			if task.repeated && !task.cancelled.Load() {
				task.execAt = task.execAt.Add(task.interval)
				if !task.cancelled.Load() && s.queue.AddTask(task) {
					s.wakeupLoop()
				}
			}
		}

		s.resetTimer(s.queue.NextExecDuration(time.Now()))

		select {
		case <-s.timer.C:
		case <-s.wakeup:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *heapScheduler) Once(delay time.Duration, f func()) int64 {
	return s.scheduleAt(time.Now().Add(delay), 0, false, f)
}

func (s *heapScheduler) ScheduleAt(execTime time.Time, f func()) int64 {
	return s.scheduleAt(execTime, 0, false, f)
}

func (s *heapScheduler) Forever(interval time.Duration, f func()) int64 {
	return s.scheduleAt(time.Now().Add(interval), interval, true, f)
}

func (s *heapScheduler) ForeverNow(interval time.Duration, f func()) int64 {
	// 立即执行一次
	if s.workerPool != nil {
		s.workerPool.Submit(f)
	} else {
		go safeCall(f)
	}
	return s.scheduleAt(time.Now().Add(interval), interval, true, f)
}

func (s *heapScheduler) Cancel(taskID int64) {
	s.queue.RemoveTask(taskID)
	s.wakeupLoop()
}

func (s *heapScheduler) CancelAll() {
	s.queue.mu.Lock()
	defer s.queue.mu.Unlock()
	// 标记所有任务为取消，避免正在执行的任务panic
	for _, t := range s.queue.tasks {
		t.cancelled.Store(true)
	}
	s.queue.heap = []*heapTaskEntry{}
	s.queue.tasks = make(map[int64]*heapTaskEntry)
	s.wakeupLoop()
}

func (s *heapScheduler) Stop() {
	if !s.shutdown.CompareAndSwap(false, true) {
		return
	}
	s.cancel()
	s.CancelAll()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		log.Debug("[heapScheduler] stopped successfully")
	case <-time.After(3 * time.Second):
		log.Warn("[heapScheduler] shutdown timed out after 3s")
	}

	// 最后停止workerPool（此时所有任务都已完成）
	if s.workerPool != nil {
		s.workerPool.Stop()
	}
}

func (s *heapScheduler) scheduleAt(execTime time.Time, interval time.Duration, repeated bool, f func()) int64 {
	if s.shutdown.Load() || s.ctx.Err() != nil {
		log.Warn("[heapScheduler] is shut down; task rejected")
		return -1
	}
	if f == nil {
		log.Warn("[heapScheduler] task function is nil; task rejected")
		return -1
	}

	// 检查执行时间是否早于当前时间（允许过期任务，但会立即执行）
	now := time.Now()
	if execTime.Before(now) {
		if repeated {
			// 周期任务：如果执行时间已过期，从当前时间开始计算
			execTime = now.Add(interval)
		} else {
			// 一次性任务：过期任务会被立即执行（通过PopExpired处理）
			log.Debugf("[heapScheduler] task scheduled at past time %v, will execute immediately", execTime)
		}
	}

	taskID := s.nextID.Add(1)
	t := &heapTaskEntry{
		id:       taskID,
		execAt:   execTime,
		interval: interval,
		repeated: repeated,
		task:     f,
	}
	if s.queue.AddTask(t) {
		s.wakeupLoop()
	}
	return taskID
}

func (s *heapScheduler) wakeupLoop() {
	select {
	case s.wakeup <- struct{}{}:
	default:
	}
}

func (s *heapScheduler) resetTimer(d time.Duration) {
	if !s.timer.Stop() {
		select {
		case <-s.timer.C:
		default:
		}
	}
	// 如果duration为0或负数，设置为最小间隔（有过期任务需要立即处理）
	if d <= 0 {
		d = time.Millisecond
	}
	s.timer.Reset(d)
}

// ==================== WorkerPool ====================

// WorkerPool 工作协程池
type WorkerPool struct {
	tasks    chan func()
	stopOnce sync.Once
	closed   uint32
	wg       sync.WaitGroup
}

// NewWorkerPool 创建固定大小的 WorkerPool
func NewWorkerPool(size, queueSize int) *WorkerPool {
	if size <= 0 {
		return nil
	}
	if queueSize <= 0 {
		queueSize = 1024 // 默认队列大小
	}
	p := &WorkerPool{
		tasks: make(chan func(), queueSize),
	}
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for task := range p.tasks {
				if task == nil {
					continue
				}
				safeCall(task)
			}
		}()
	}
	return p
}

// Submit 提交任务到 WorkerPool，满或关闭降级为 goroutine
func (p *WorkerPool) Submit(task func()) {
	if task == nil {
		return
	}
	if p == nil || atomic.LoadUint32(&p.closed) == 1 {
		// pool 已关闭或 nil，直接启动 goroutine 执行
		go safeCall(task)
		return
	}
	select {
	case p.tasks <- task:
	default:
		// 队列满，降级执行
		go safeCall(task)
	}
}

// Stop 停止 WorkerPool
func (p *WorkerPool) Stop() {
	if p == nil {
		return
	}
	p.stopOnce.Do(func() {
		atomic.StoreUint32(&p.closed, 1)
		close(p.tasks)
		p.wg.Wait()
	})
}

// safeCall 包装任务执行，捕获 panic
func safeCall(fn func()) {
	if fn == nil {
		return
	}
	defer RecoverFromError(nil)
	fn()
}

// RecoverFromError 捕获 panic 并打印堆栈
func RecoverFromError(cb func(e any)) {
	if e := recover(); e != nil {
		log.Errorf("Recover => %v\n%s\n", e, debug.Stack())
		if cb != nil {
			cb(e)
		}
	}
}
