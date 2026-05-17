// demosim 为 doc/demo.py 的 Go 移植，用法见同目录 README。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

var (
	flagMode   = flag.String("mode", "BASE", "仿真模式: BASE 或 BUY")
	flagRounds = flag.Int64("n", 100000000, "仿真局数")
	flagConfig = flag.String("config", "", "config.json 路径，默认自动查找")
)

type demoConfig struct {
	GameConfig struct {
		Columns      int     `json:"columns"`
		Rows         int     `json:"rows"`
		WildID       int     `json:"wild_id"`
		ScID         int     `json:"sc_id"`
		MultiplierID int     `json:"multiplier_id"`
		BaseBet      float64 `json:"base_bet"`
		WildMaxLimit int     `json:"wild_max_limit"`
		MaxWinCap    float64 `json:"max_win_cap"`
	} `json:"game_config"`
	SymbolPayouts map[string]map[string]float64 `json:"symbol_payouts"`
	ModesConfig   map[string]modeConfig         `json:"modes_config"`
	ReelSets      map[string]json.RawMessage    `json:"reel_sets"`
}

type modeConfig struct {
	ReelKey        string         `json:"reel_key"`
	ReelConfigs    []reelCfg      `json:"reel_configs"`
	InitialSpins   int            `json:"initial_spins"`
	RetriggerCount int            `json:"retrigger_count"`
	RetriggerAdd   int            `json:"retrigger_add"`
	InitialScatter map[string]int `json:"initial_scatter"`
	Multiplier     multiplierConf `json:"multiplier"`
	WildGen        wildGenConf    `json:"wild_gen"`
}

type reelCfg struct {
	ReelKey string `json:"reel_key"`
	Weight  int    `json:"weight"`
}

type multiplierConf struct {
	ProbPerCol float64        `json:"prob_per_col"`
	Weight     map[string]int `json:"weight"`
}

// discreteWeights 对齐 doc/game.json：支持 []int（下标 i=档位 i，元素=权重）或 JSON map（按键数值排序）。
type discreteWeights struct {
	keys    []int
	weights []int
}

func (d *discreteWeights) UnmarshalJSON(b []byte) error {
	d.keys = nil
	d.weights = nil
	var arr []int
	if err := json.Unmarshal(b, &arr); err == nil {
		if len(arr) == 0 {
			return nil
		}
		d.keys = make([]int, len(arr))
		d.weights = make([]int, len(arr))
		for i, w := range arr {
			d.keys[i] = i
			d.weights[i] = w
		}
		return nil
	}
	var m map[string]int
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	type kv struct {
		k, w int
	}
	var rows []kv
	for ks, w := range m {
		k, err := strconv.Atoi(ks)
		if err != nil {
			return fmt.Errorf("wild_gen: invalid key %q", ks)
		}
		rows = append(rows, kv{k, w})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].k < rows[j].k })
	for _, row := range rows {
		d.keys = append(d.keys, row.k)
		d.weights = append(d.weights, row.w)
	}
	return nil
}

func (d discreteWeights) pick(rng *rand.Rand) int {
	if len(d.keys) == 0 {
		return 0
	}
	i := pickWeightedSlice(d.weights, rng)
	return d.keys[i]
}

type wildGenConf struct {
	InitialSpawn discreteWeights `json:"initial_spawn"`
	TumbleRefill discreteWeights `json:"tumble_refill"`
}

type spinRes struct {
	pureWin, wildWin float64
	scCount          int
	winMTotal        int64
	isMaxWin         bool
	mBallCount       int
	tumbles          int
	wildCount        int
	wildExplodes     int
	symbolRawWins    map[int]float64
}

type fgDetails struct {
	win, wWin, pWin     float64
	balls, maxHits      int
	totalSpins          int
	mTotalSum           int64
	fgWinSpins          int
	symbolContributions map[int]float64
}

type metrics struct {
	basePW, baseWW, baseSC float64
	fgTW, fgPW, fgWW       float64
	maxH, trigs            int64
	fgSpinsTotal           int64
	fgBalls                int64
	baseRounds             int64
	baseWinTimes           int64
	fgRounds               int64
	fgWinTimes             int64
}

type simulator struct {
	cfg     demoConfig
	reels   map[string][][]int
	cols    int
	rows    int
	wildID  int
	scID    int
	mBall   int
	baseBet float64
	wildMax int
	maxCap  float64
	payouts map[string]map[string]float64
}

func main() {
	flag.Parse()
	mode := *flagMode
	if mode != "BASE" && mode != "BUY" {
		fmt.Fprintf(os.Stderr, "mode must be BASE or BUY\n")
		os.Exit(1)
	}
	rounds := *flagRounds
	if rounds <= 0 {
		fmt.Fprintf(os.Stderr, "n must be > 0\n")
		os.Exit(1)
	}

	cfgPath, err := resolveConfigPath(*flagConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read config: %v\n", err)
		os.Exit(1)
	}
	var cfg demoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "parse config: %v\n", err)
		os.Exit(1)
	}
	sim := newSimulator(cfg)

	workers := runtime.NumCPU()
	tasks := 1000
	chunk := int(rounds / int64(tasks))
	if chunk < 1 {
		tasks = int(rounds)
		chunk = 1
	}

	var merged metrics
	var mu sync.Mutex
	var done int64

	fmt.Printf("demosim | mode=%s | n=%d | workers=%d | config=%s\n", mode, rounds, workers, cfgPath)

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, workers)
	for i := 0; i < tasks; i++ {
		wg.Add(1)
		sem <- struct{}{}
		seed := uint64(i + 1)
		n := chunk
		if i == tasks-1 && int64(tasks)*int64(chunk) < rounds {
			n += int(rounds - int64(tasks)*int64(chunk))
		}
		go func(n int) {
			defer wg.Done()
			defer func() { <-sem }()
			rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
			m := runChunk(sim, n, mode, rng)
			mu.Lock()
			mergeMetrics(&merged, &m)
			mu.Unlock()
			cur := atomic.AddInt64(&done, 1)
			if cur%100 == 0 || cur == int64(tasks) {
				fmt.Printf("\rprogress: %d/%d chunks", cur, tasks)
			}
		}(n)
	}
	wg.Wait()
	fmt.Println()
	printReport(&merged, &cfg, mode, rounds)
}

func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	candidates := []string{
		"config.json",
		filepath.Join("..", "config.json"),
		filepath.Join("doc", "config.json"),
		filepath.Join("game", "tmtg", "doc", "config.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("config.json not found; use -config")
}

func newSimulator(cfg demoConfig) *simulator {
	reels := make(map[string][][]int, len(cfg.ReelSets))
	for k, raw := range cfg.ReelSets {
		var cols [][]int
		if err := json.Unmarshal(raw, &cols); err != nil {
			continue
		}
		reels[k] = cols
	}
	return &simulator{
		cfg:     cfg,
		reels:   reels,
		cols:    cfg.GameConfig.Columns,
		rows:    cfg.GameConfig.Rows,
		wildID:  cfg.GameConfig.WildID,
		scID:    cfg.GameConfig.ScID,
		mBall:   cfg.GameConfig.MultiplierID,
		baseBet: cfg.GameConfig.BaseBet,
		wildMax: cfg.GameConfig.WildMaxLimit,
		maxCap:  cfg.GameConfig.MaxWinCap,
		payouts: cfg.SymbolPayouts,
	}
}

func runChunk(sim *simulator, n int, mode string, rng *rand.Rand) metrics {
	var m metrics
	for i := 0; i < n; i++ {
		if mode == "BASE" {
			accumBaseChunk(sim, rng, &m)
		} else {
			accumBuyChunk(sim, rng, &m)
		}
	}
	return m
}

func accumBaseChunk(sim *simulator, rng *rand.Rand, m *metrics) {
	res := sim.playBase(rng)
	m.baseRounds++
	if res.pureWin+res.wildWin+res.scPay > 0 {
		m.baseWinTimes++
	}
	if !res.fgTrig {
		m.basePW += res.pureWin
		m.baseWW += res.wildWin
		return
	}
	// 触发局线赢不在 _run_free_logic 的 win 内，须计入 base（与 demo.py、Go TestRtp 一致）
	m.basePW += res.pureWin
	m.baseWW += res.wildWin
	m.trigs++
	d := res.fgDetails
	m.fgTW += d.win
	m.fgPW += d.pWin
	m.fgWW += d.wWin
	m.fgBalls += int64(d.balls)
	m.fgSpinsTotal += int64(d.totalSpins)
	m.fgRounds += int64(d.totalSpins)
	m.fgWinTimes += int64(d.fgWinSpins)
	m.maxH += int64(d.maxHits)
}

func accumBuyChunk(sim *simulator, rng *rand.Rand, m *metrics) {
	res := sim.playBuy(rng)
	m.trigs++
	d := res.details
	m.fgTW += d.win
	m.fgPW += d.pWin
	m.fgWW += d.wWin
	m.fgBalls += int64(d.balls)
	m.fgSpinsTotal += int64(d.totalSpins)
	m.fgRounds += int64(d.totalSpins)
	m.fgWinTimes += int64(d.fgWinSpins)
	m.maxH += int64(d.maxHits)
}

func (s *simulator) playBase(rng *rand.Rand) struct {
	pureWin, wildWin, scPay float64
	fgTrig                  bool
	fgDetails               *fgDetails
} {
	b := s.runOneSpin("base_game", rng)
	scPay := scatterPay(s, b.scCount)
	out := struct {
		pureWin, wildWin, scPay float64
		fgTrig                  bool
		fgDetails               *fgDetails
	}{b.pureWin, b.wildWin, scPay, false, nil}
	if b.scCount >= 4 {
		out.fgTrig = true
		out.fgDetails = s.runFreeLogic("free_game", rng, scPay)
	}
	return out
}

func (s *simulator) playBuy(rng *rand.Rand) struct {
	details *fgDetails
} {
	conf := s.cfg.ModesConfig["free_buy"]
	scC := pickWeighted(conf.InitialScatter, rng)
	scPay := scatterPay(s, scC)
	return struct{ details *fgDetails }{
		details: s.runFreeLogic("free_buy", rng, scPay),
	}
}

func scatterPay(s *simulator, scCount int) float64 {
	if scCount < 4 {
		return 0
	}
	pays, ok := s.payouts[fmt.Sprint(s.scID)]
	if !ok {
		return 0
	}
	key := fmt.Sprint(min(scCount, 6))
	return pays[key]
}

func (s *simulator) runFreeLogic(modeKey string, rng *rand.Rand, initialPay float64) *fgDetails {
	conf := s.cfg.ModesConfig[modeKey]
	spins := conf.InitialSpins
	totalWin := initialPay
	var wWin, pWin float64
	var balls, maxHits, totalSpins, fgWinSpins int
	var totalMSum int64
	symWins := map[int]float64{}
	if initialPay > 0 {
		symWins[s.scID] += initialPay
	}
	if totalWin/s.baseBet >= s.maxCap {
		totalWin = s.maxCap * s.baseBet
		maxHits, spins = 1, 0
	}
	for spins > 0 {
		spins--
		totalSpins++
		f := s.runOneSpin(modeKey, rng)
		thisWin := f.pureWin + f.wildWin
		if thisWin > 0 {
			fgWinSpins++
		}
		if (totalWin+thisWin)/s.baseBet >= s.maxCap {
			remaining := s.maxCap*s.baseBet - totalWin
			if thisWin > 0 {
				ratio := remaining / thisWin
				totalWin = s.maxCap * s.baseBet
				wWin += f.wildWin * ratio
				pWin += f.pureWin * ratio
				for sid, val := range f.symbolRawWins {
					symWins[sid] += val * ratio
				}
			} else {
				totalWin = s.maxCap * s.baseBet
			}
			maxHits = 1
			break
		}
		totalWin += thisWin
		wWin += f.wildWin
		pWin += f.pureWin
		for sid, val := range f.symbolRawWins {
			symWins[sid] += val
		}
		balls += f.mBallCount
		if f.winMTotal > 0 {
			totalMSum += f.winMTotal
		}
		if f.scCount >= conf.RetriggerCount {
			spins += conf.RetriggerAdd
		}
	}
	return &fgDetails{
		win: totalWin, wWin: wWin, pWin: pWin, balls: balls, maxHits: maxHits,
		totalSpins: totalSpins, mTotalSum: totalMSum, fgWinSpins: fgWinSpins,
		symbolContributions: symWins,
	}
}

func (s *simulator) runOneSpin(modeKey string, rng *rand.Rand) spinRes {
	p := s.getModeParams(modeKey, rng)
	reels := p.reels
	grid := make([][]int, s.cols)
	top := make([]int, s.cols)
	for c := 0; c < s.cols; c++ {
		grid[c] = make([]int, s.rows)
		rlen := len(reels[c])
		t := rng.IntN(rlen)
		top[c] = t
		for r := 0; r < s.rows; r++ {
			grid[c][r] = reels[c][(t+r)%rlen]
		}
	}
	res := spinRes{symbolRawWins: map[int]float64{}}
	mValues := map[[2]int]int64{}

	currW := countGrid(grid, s.wildID)
	if currW < s.wildMax {
		target := p.wildInit.pick(rng)
		var eligible [][2]int
		for c := 0; c < s.cols; c++ {
			for r := 0; r < s.rows; r++ {
				sym := grid[c][r]
				if sym != s.scID && sym != s.mBall && sym != s.wildID {
					eligible = append(eligible, [2]int{c, r})
				}
			}
		}
		n := min3(target, s.wildMax-currW, len(eligible))
		shufflePairs(eligible, rng)
		for i := 0; i < n; i++ {
			grid[eligible[i][0]][eligible[i][1]] = s.wildID
		}
	}

	for c := 0; c < s.cols; c++ {
		if rng.Float64() >= p.mProb {
			continue
		}
		var rows []int
		for r := 0; r < s.rows; r++ {
			sym := grid[c][r]
			if sym != s.scID && sym != s.wildID {
				rows = append(rows, r)
			}
		}
		if len(rows) == 0 {
			continue
		}
		tr := rows[rng.IntN(len(rows))]
		grid[c][tr] = s.mBall
		mValues[[2]int{c, tr}] = int64(pickWeightedVal(p.mVals, p.mWeights, rng))
	}

	var spinP, spinW float64
	for {
		counts := countAll(grid)
		wildCount := counts[s.wildID]
		mask := make([][]bool, s.cols)
		for c := 0; c < s.cols; c++ {
			mask[c] = make([]bool, s.rows)
		}
		var pStep, wStep float64
		isWin, isExplode := false, false

		for sidStr, pays := range s.payouts {
			var sid int
			fmt.Sscan(sidStr, &sid)
			if sid == s.scID || sid == s.mBall || sid == s.wildID {
				continue
			}
			total := counts[sid] + wildCount
			if total < 8 {
				continue
			}
			isWin = true
			winVal := payoutVal(pays, total)
			for c := 0; c < s.cols; c++ {
				for r := 0; r < s.rows; r++ {
					if grid[c][r] == sid {
						mask[c][r] = true
					}
				}
			}
			res.symbolRawWins[sid] += winVal
			if wildCount > 0 {
				wStep += winVal
			} else {
				pStep += winVal
			}
		}

		if !isWin && modeKey != "base_game" && wildCount > 0 {
			isExplode = true
			res.wildExplodes++
			for c := 0; c < s.cols; c++ {
				for r := 0; r < s.rows; r++ {
					if grid[c][r] != s.wildID {
						continue
					}
					for rr := 0; rr < s.rows; rr++ {
						mask[c][rr] = true
					}
					for cc := 0; cc < s.cols; cc++ {
						mask[cc][r] = true
					}
				}
			}
			for c := 0; c < s.cols; c++ {
				for r := 0; r < s.rows; r++ {
					if grid[c][r] == s.mBall || grid[c][r] == s.scID {
						mask[c][r] = false
					}
					if grid[c][r] == s.wildID {
						mask[c][r] = true
					}
				}
			}
		}

		if !isWin && !isExplode {
			break
		}
		res.tumbles++
		spinP += pStep
		spinW += wStep
		newM := map[[2]int]int64{}
		for c := 0; c < s.cols; c++ {
			var rem [][2]interface{}
			for r := 0; r < s.rows; r++ {
				if !mask[c][r] {
					rem = append(rem, [2]interface{}{grid[c][r], mValues[[2]int{c, r}]})
				}
			}
			needed := s.rows - len(rem)
			var newE [][2]interface{}
			if needed > 0 {
				rlen := len(reels[c])
				var newIDs []int
				for i := 1; i <= needed; i++ {
					idx := top[c] - i
					idx %= rlen
					if idx < 0 {
						idx += rlen
					}
					newIDs = append(newIDs, reels[c][idx])
				}
				for i, j := 0, len(newIDs)-1; i < j; i, j = i+1, j-1 {
					newIDs[i], newIDs[j] = newIDs[j], newIDs[i]
				}
				top[c] = (top[c] - needed) % rlen
				if top[c] < 0 {
					top[c] += rlen
				}
				for _, sid := range newIDs {
					newE = append(newE, [2]interface{}{sid, nil})
				}
				hasBombRem := false
				for _, x := range rem {
					if x[0].(int) == s.mBall {
						hasBombRem = true
						break
					}
				}
				if !hasBombRem && rng.Float64() < p.mProb {
					var v []int
					for i, x := range newE {
						sid := x[0].(int)
						if sid != s.scID && sid != s.wildID {
							v = append(v, i)
						}
					}
					if len(v) > 0 {
						idx := v[rng.IntN(len(v))]
						newE[idx][0] = s.mBall
						newE[idx][1] = int64(pickWeightedVal(p.mVals, p.mWeights, rng))
					}
				}
				tw := p.wildRefill.pick(rng)
				gridWild := countGrid(grid, s.wildID)
				newEWild := 0
				for _, x := range newE {
					if x[0].(int) == s.wildID {
						newEWild++
					}
				}
				wildLimit := tw
				if rem := s.wildMax - gridWild - newEWild; rem < wildLimit {
					wildLimit = rem
				}
				for k := 0; k < wildLimit; k++ {
					var vw []int
					for i, x := range newE {
						sid := x[0].(int)
						if sid != s.scID && sid != s.mBall && sid != s.wildID {
							vw = append(vw, i)
						}
					}
					if len(vw) == 0 {
						break
					}
					idx := vw[rng.IntN(len(vw))]
					newE[idx][0] = s.wildID
					newEWild++
				}
			}
			all := append(newE, rem...)
			for rIdx, pair := range all {
				sid := pair[0].(int)
				grid[c][rIdx] = sid
				if sid == s.wildID {
					res.wildCount++
				}
				if sid == s.mBall {
					var mv int64
					if pair[1] != nil {
						mv = pair[1].(int64)
					} else if old, ok := mValues[[2]int{c, rIdx}]; ok {
						mv = old
					} else {
						mv = int64(pickWeightedVal(p.mVals, p.mWeights, rng))
					}
					newM[[2]int{c, rIdx}] = mv
				}
			}
		}
		mValues = newM
	}

	res.scCount = countGrid(grid, s.scID)
	sumM := int64(0)
	for _, v := range mValues {
		sumM += v
	}
	if modeKey != "base_game" && spinP+spinW > 0 && sumM > 0 {
		res.winMTotal = sumM
		fWin := (spinP + spinW) * float64(sumM)
		if fWin/s.baseBet >= s.maxCap {
			fWin = s.maxCap * s.baseBet
			res.isMaxWin = true
		}
		den := spinP + spinW
		rat := 1.0
		if den > 0 {
			rat = spinP / den
		}
		res.pureWin = fWin * rat
		res.wildWin = fWin * (1 - rat)
	} else {
		res.pureWin = spinP
		res.wildWin = spinW
	}
	res.mBallCount = len(mValues)
	return res
}

type modeParams struct {
	reels      [][]int
	mProb      float64
	mVals      []int
	mWeights   []int
	wildInit   discreteWeights
	wildRefill discreteWeights
}

func (s *simulator) getModeParams(modeKey string, rng *rand.Rand) modeParams {
	conf := s.cfg.ModesConfig[modeKey]
	reels := s.selectReels(conf, rng)
	md := conf.Multiplier
	vals, weights := multiplierValsWeightsSorted(md.Weight)
	return modeParams{
		reels: reels, mProb: md.ProbPerCol, mVals: vals, mWeights: weights,
		wildInit: conf.WildGen.InitialSpawn, wildRefill: conf.WildGen.TumbleRefill,
	}
}

func multiplierValsWeightsSorted(wmap map[string]int) (vals []int, weights []int) {
	type kv struct {
		v, wt int
	}
	var rows []kv
	for ks, wt := range wmap {
		v, err := strconv.Atoi(ks)
		if err != nil {
			continue
		}
		rows = append(rows, kv{v, wt})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].v < rows[j].v })
	for _, row := range rows {
		vals = append(vals, row.v)
		weights = append(weights, row.wt)
	}
	return vals, weights
}

func (s *simulator) selectReels(conf modeConfig, rng *rand.Rand) [][]int {
	if len(conf.ReelConfigs) == 0 {
		return s.reels[conf.ReelKey]
	}
	var keys []string
	var weights []int
	for _, r := range conf.ReelConfigs {
		keys = append(keys, r.ReelKey)
		weights = append(weights, r.Weight)
	}
	return s.reels[keys[pickWeightedSlice(weights, rng)]]
}

func printReport(m *metrics, cfg *demoConfig, mode string, rounds int64) {
	bTotal := m.basePW + m.baseWW + m.baseSC
	fTotal := m.fgTW
	totalWin := bTotal + fTotal
	cost := float64(rounds) * cfg.GameConfig.BaseBet
	if mode == "BUY" {
		cost *= 100
	}
	tTrig := m.trigs
	if tTrig == 0 {
		tTrig = 1
	}

	fmt.Println("\n" + "=============================================================================")
	fmt.Printf("║     SWEET WILD 仿真报告 (demosim / demo.py)  mode=%-4s  n=%-10d      ║\n", mode, rounds)
	fmt.Println("=============================================================================")
	fmt.Println("【整体经济指标】")
	fmt.Printf("  > 总返还 (RTP)  : %10.2f %%\n", totalWin/cost*100)
	fmt.Printf("  > Base 贡献 RTP : %10.2f %%\n", bTotal/cost*100)
	fmt.Printf("  > Free 贡献 RTP : %10.2f %%\n", fTotal/cost*100)
	if mode == "BASE" {
		fmt.Printf("  > FG 触发频率   : 1 / %.1f 转\n", float64(rounds)/float64(tTrig))
		fmt.Printf("  > Base 中奖率   : %10.4f %%  (%d/%d)\n",
			float64(m.baseWinTimes)/float64(max64(m.baseRounds, 1))*100, m.baseWinTimes, m.baseRounds)
	}
	if m.fgRounds > 0 {
		fmt.Printf("  > Free 中奖率   : %10.4f %%  (%d/%d)\n",
			float64(m.fgWinTimes)/float64(m.fgRounds)*100, m.fgWinTimes, m.fgRounds)
	}
	fmt.Printf("  > MaxWin 触发数 : %d (1 / %.0f 转)\n", m.maxH, float64(rounds)/float64(max64(m.maxH, 1)))
	fmt.Println("-----------------------------------------------------------------------------")
	if mode == "BASE" {
		fmt.Printf("Runtime=%d baseRtp=%.4f%% baseWinRate=%.4f%% freeRtp=%.4f%% freeWinRate=%.4f%% freeTriggerRate=%.4f%% avgFree=%.4f Rtp=%.4f%%\n",
			m.baseRounds,
			(m.basePW+m.baseWW)/cost*100,
			float64(m.baseWinTimes)/float64(max64(m.baseRounds, 1))*100,
			fTotal/cost*100,
			float64(m.fgWinTimes)/float64(max64(m.fgRounds, 1))*100,
			float64(m.trigs)/float64(max64(m.baseRounds, 1))*100,
			float64(m.fgSpinsTotal)/float64(tTrig),
			totalWin/cost*100,
		)
		fmt.Printf("totalWin=%.0f freeWin=%.0f baseWin=%.0f baseWinTime=%d freeTrig=%d freeRound=%d freeWinTime=%d\n",
			totalWin, fTotal, m.basePW+m.baseWW, m.baseWinTimes, m.trigs, m.fgRounds, m.fgWinTimes)
	} else {
		fmt.Printf("Rtp=%.4f%% freeWinRate=%.4f%% avgFreeSpin=%.4f totalWin=%.0f\n",
			totalWin/cost*100,
			float64(m.fgWinTimes)/float64(max64(m.fgRounds, 1))*100,
			float64(m.fgSpinsTotal)/float64(tTrig),
			totalWin,
		)
	}
	fmt.Println("=============================================================================")
}

func mergeMetrics(dst, src *metrics) {
	dst.basePW += src.basePW
	dst.baseWW += src.baseWW
	dst.baseSC += src.baseSC
	dst.fgTW += src.fgTW
	dst.fgPW += src.fgPW
	dst.fgWW += src.fgWW
	dst.maxH += src.maxH
	dst.trigs += src.trigs
	dst.fgSpinsTotal += src.fgSpinsTotal
	dst.fgBalls += src.fgBalls
	dst.baseRounds += src.baseRounds
	dst.baseWinTimes += src.baseWinTimes
	dst.fgRounds += src.fgRounds
	dst.fgWinTimes += src.fgWinTimes
}

func countGrid(grid [][]int, sym int) int {
	n := 0
	for c := range grid {
		for r := range grid[c] {
			if grid[c][r] == sym {
				n++
			}
		}
	}
	return n
}

func countAll(grid [][]int) map[int]int {
	m := map[int]int{}
	for c := range grid {
		for r := range grid[c] {
			m[grid[c][r]]++
		}
	}
	return m
}

func payoutVal(pays map[string]float64, total int) float64 {
	key := "12+"
	if total <= 9 {
		key = "8-9"
	} else if total <= 11 {
		key = "10-11"
	}
	return pays[key]
}

// pickWeighted 对 map 按键数值升序排列后再抽样（与无序 range 无关，便于与 game_json 对齐、复现）。
func pickWeighted(m map[string]int, rng *rand.Rand) int {
	if len(m) == 0 {
		return 0
	}
	type kv struct {
		k, w int
	}
	var rows []kv
	for ks, w := range m {
		k, err := strconv.Atoi(ks)
		if err != nil {
			continue
		}
		rows = append(rows, kv{k, w})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].k < rows[j].k })
	if len(rows) == 0 {
		return 0
	}
	keys := make([]int, len(rows))
	wts := make([]int, len(rows))
	for i := range rows {
		keys[i], wts[i] = rows[i].k, rows[i].w
	}
	return keys[pickWeightedSlice(wts, rng)]
}

func pickWeightedVal(vals, weights []int, rng *rand.Rand) int {
	if len(vals) == 0 {
		return 0
	}
	return vals[pickWeightedSlice(weights, rng)]
}

func pickWeightedSlice(weights []int, rng *rand.Rand) int {
	sum := 0
	for _, w := range weights {
		sum += w
	}
	if sum <= 0 {
		return 0
	}
	x := rng.IntN(sum)
	for i, w := range weights {
		if x < w {
			return i
		}
		x -= w
	}
	return len(weights) - 1
}

func shufflePairs(p [][2]int, rng *rand.Rand) {
	rng.Shuffle(len(p), func(i, j int) { p[i], p[j] = p[j], p[i] })
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
