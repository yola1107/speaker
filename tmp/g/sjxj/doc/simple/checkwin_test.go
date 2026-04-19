package main

import "testing"

func TestCheckWin_Line0_Length5_AllMatch(t *testing.T) {
	// line0 from missworld.json is [20,21,22,23,24]
	// With Cols=5, this maps to row=4 and col=0..4.
	var grid Matrix
	for c := 0; c < Cols; c++ {
		grid[4][c] = 2 // originSymbol=2 (J)
	}

	details, totalWin, _ := CheckWin(grid)
	expectedLineWin := PayTableJSON[2-1][5-1] // PayTableJSON[symbol-1][lineLen-1]

	found := false
	for _, d := range details {
		if d.LineIndex == 0 && d.LineLen == 5 {
			found = true
			if d.Win != expectedLineWin {
				t.Fatalf("line0 win=%d expected=%d (totalWin=%d)", d.Win, expectedLineWin, totalWin)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected a length-5 win on line0, but not found (totalWin=%d)", totalWin)
	}
}

func TestCheckWin_Line0_WildSubstitute(t *testing.T) {
	// originSymbol=2 at (row4,col0); other stops are wild so line length should be 5.
	var grid Matrix
	grid[4][0] = 2
	for c := 1; c < Cols; c++ {
		grid[4][c] = Wild
	}

	details, totalWin, _ := CheckWin(grid)
	expectedLineWin := PayTableJSON[2-1][5-1]

	found := false
	for _, d := range details {
		if d.LineIndex == 0 && d.LineLen == 5 {
			found = true
			if d.Win != expectedLineWin {
				t.Fatalf("line0 win=%d expected=%d (totalWin=%d)", d.Win, expectedLineWin, totalWin)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected a length-5 win on line0 with wild substitution, but not found (totalWin=%d)", totalWin)
	}
}

func TestCheckWin_UserCase_Line10And11(t *testing.T) {
	// User-provided final grid:
	// [1 3 2 4 2]
	// [6 6 1 1 7]
	// [4 4 3 5 4]
	// [1 7 6 2 4]
	// [2 2 6 1 1]
	// [3 5 6 7 1]
	// [4 4 2 3 3]
	// [11 5 3 7 4]
	grid := Matrix{
		{1, 3, 2, 4, 2},
		{6, 6, 1, 1, 7},
		{4, 4, 3, 5, 4},
		{1, 7, 6, 2, 4},
		{2, 2, 6, 1, 1},
		{3, 5, 6, 7, 1},
		{4, 4, 2, 3, 3},
		{11, 5, 3, 7, 4},
	}

	details, totalWin, _ := CheckWin(grid)

	if totalWin != 4 {
		t.Fatalf("totalWin=%d expected=4, details=%v", totalWin, details)
	}
	if len(details) != 2 {
		t.Fatalf("details len=%d expected=2, details=%v", len(details), details)
	}

	// Expected (0-based line index):
	// line 9  -> [20,21,32,23,24] => rows [4,4,6,4,4], first 3 are symbol 2
	// line 10 -> [20,21,32,28,24] => rows [4,4,6,5,4], first 3 are symbol 2
	expected := map[int]bool{9: false, 10: false}
	for _, d := range details {
		if d.SymbolID != 2 || d.LineLen != 3 || d.Win != 2 {
			t.Fatalf("unexpected detail=%v, expected symbol=2 len=3 win=2", d)
		}
		if _, ok := expected[d.LineIndex]; ok {
			expected[d.LineIndex] = true
		}
	}
	for idx, ok := range expected {
		if !ok {
			t.Fatalf("expected line index %d not found, details=%v", idx, details)
		}
	}
}
