package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	// 11个符号对应权重：10,J,Q,K,A,HighHeel,Ribbon,Scepter,Crown,Wild,Scatter
	defaultReelWeights = []int{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5}
	defaultReelSymbols = []int{
		Sym10, SymJ, SymQ, SymK, SymA,
		SymHighHeel, SymRibbon, SymScepter, SymCrown,
		Wild, Scatter,
	}
)

type remainderEntry struct {
	idx       int
	remainder float64
}

func buildSymbolCountsByWeights(reelLen int, weights []int) ([]int, error) {
	if reelLen <= 0 {
		return nil, fmt.Errorf("reelLen must be > 0")
	}
	if len(weights) == 0 {
		return nil, fmt.Errorf("weights must not be empty")
	}

	totalWeight := 0
	for _, w := range weights {
		if w < 0 {
			return nil, fmt.Errorf("weights must be >= 0")
		}
		totalWeight += w
	}
	if totalWeight == 0 {
		return nil, fmt.Errorf("weights total must be > 0")
	}

	counts := make([]int, len(weights))
	entries := make([]remainderEntry, 0, len(weights))
	used := 0

	for i, w := range weights {
		exact := float64(w) * float64(reelLen) / float64(totalWeight)
		base := int(math.Floor(exact))
		counts[i] = base
		used += base
		entries = append(entries, remainderEntry{
			idx:       i,
			remainder: exact - float64(base),
		})
	}

	remain := reelLen - used
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].remainder == entries[j].remainder {
			return entries[i].idx < entries[j].idx
		}
		return entries[i].remainder > entries[j].remainder
	})
	for i := 0; i < remain; i++ {
		counts[entries[i].idx]++
	}

	return counts, nil
}

func generateWeightedReelStrip(reelLen int, symbols []int, weights []int, r *rand.Rand) ([]int, error) {
	if len(symbols) != len(weights) {
		return nil, fmt.Errorf("symbols and weights length mismatch")
	}
	counts, err := buildSymbolCountsByWeights(reelLen, weights)
	if err != nil {
		return nil, err
	}

	strip := make([]int, 0, reelLen)
	for i, c := range counts {
		for n := 0; n < c; n++ {
			strip = append(strip, symbols[i])
		}
	}

	if len(strip) != reelLen {
		return nil, fmt.Errorf("generated strip length mismatch: got %d want %d", len(strip), reelLen)
	}

	r.Shuffle(len(strip), func(i, j int) {
		strip[i], strip[j] = strip[j], strip[i]
	})
	return strip, nil
}

func generateRealDataWithSharedWeights(seed int64) ([][][]int, error) {
	r := rand.New(rand.NewSource(seed))

	baseReels := make([][]int, Cols)
	freeReels := make([][]int, Cols)

	for col := 0; col < Cols; col++ {
		baseStrip, err := generateWeightedReelStrip(ReelLen, defaultReelSymbols, defaultReelWeights, r)
		if err != nil {
			return nil, err
		}
		freeStrip, err := generateWeightedReelStrip(ReelLen, defaultReelSymbols, defaultReelWeights, r)
		if err != nil {
			return nil, err
		}
		baseReels[col] = baseStrip
		freeReels[col] = freeStrip
	}

	return [][][]int{baseReels, freeReels}, nil
}

// GenerateAndWriteReelsToJSON 按统一权重生成 base/free reel，并写入 missworld.json 的 real_data。
// 返回 seed，便于复现同一套结果。
func GenerateAndWriteReelsToJSON() (int64, error) {
	seed := time.Now().UnixNano()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return 0, fmt.Errorf("runtime.Caller failed")
	}
	jsonPath := filepath.Join(filepath.Dir(thisFile), "missworld.json")

	b, err := os.ReadFile(jsonPath)
	if err != nil {
		return 0, err
	}

	var cfg missWorldJSON
	if err := json.Unmarshal(b, &cfg); err != nil {
		return 0, err
	}

	realData, err := generateRealDataWithSharedWeights(seed)
	if err != nil {
		return 0, err
	}
	cfg.RealData = realData

	out, err := formatMissWorldJSON(cfg)
	if err != nil {
		return 0, err
	}
	if err := os.WriteFile(jsonPath, out, 0o644); err != nil {
		return 0, err
	}

	return seed, nil
}

// RewriteMissWorldJSONFormat 仅重排 missworld.json 的格式，不修改任何数据内容。
func RewriteMissWorldJSONFormat() error {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("runtime.Caller failed")
	}
	jsonPath := filepath.Join(filepath.Dir(thisFile), "missworld.json")

	b, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	var cfg missWorldJSON
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}

	out, err := formatMissWorldJSON(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(jsonPath, out, 0o644)
}

func formatMissWorldJSON(cfg missWorldJSON) ([]byte, error) {
	var b strings.Builder
	b.WriteString("{\n")

	writeMatrix := func(name string, m [][]int, comma bool) {
		b.WriteString(`  "`)
		b.WriteString(name)
		b.WriteString(`": [` + "\n")
		for i, row := range m {
			b.WriteString("    ")
			b.WriteString(formatIntSlice(row))
			if i != len(m)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("  ]")
		if comma {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}

	writeMatrix("pay_table", cfg.PayTable, true)
	writeMatrix("lines", cfg.Lines, true)

	b.WriteString(`  "real_data": [` + "\n")
	for i, reelSet := range cfg.RealData {
		b.WriteString("    [\n")
		for j, strip := range reelSet {
			b.WriteString("      ")
			b.WriteString(formatIntSlice(strip))
			if j != len(reelSet)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("    ]")
		if i != len(cfg.RealData)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  ],\n")

	if len(bytes.TrimSpace(cfg.RollCfg)) > 0 {
		b.WriteString(`  "roll_cfg": `)
		b.Write(bytes.TrimSpace(cfg.RollCfg))
		b.WriteString(",\n")
	}

	b.WriteString(`  "free_game_scatter_min": ` + strconv.Itoa(cfg.FreeGameScatterMin) + ",\n")
	b.WriteString(`  "free_game_times": ` + strconv.Itoa(cfg.FreeGameTimes) + ",\n")
	writeMatrix("free_scatter_multiplier_by_row", cfg.FreeScatterMultiplierByRow, true)
	b.WriteString(`  "free_unlock_thresholds": ` + formatIntSlice(cfg.FreeUnlockThresholds) + ",\n")
	b.WriteString(`  "free_unlock_reset_spins": ` + strconv.Itoa(cfg.FreeUnlockResetSpins) + "\n")
	b.WriteString("}\n")

	return []byte(b.String()), nil
}

func formatIntSlice(vals []int) string {
	if len(vals) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range vals {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(v))
	}
	b.WriteByte(']')
	return b.String()
}
