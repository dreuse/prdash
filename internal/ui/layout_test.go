package ui

import "testing"

func TestTruncateHandlesWideAndCombiningRunes(t *testing.T) {
	tests := []struct {
		in    string
		width int
		want  string
	}{
		{"SQL idempotent", 20, "SQL idempotent"},
		{"SQL idempotent", 8, "SQL ide…"},
		{"Café résumé importer", 10, "Café résu…"},
		{"データベース移行", 16, "データベース移行"},
		{"データベース移行", 8, "データ…"},
		{"データベース移行", 7, "データ…"},
		{"anything", 0, ""},
		{"anything", 1, "a"},
	}
	for _, tc := range tests {
		got := truncate(tc.in, tc.width)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
		if w := textWidth(got); w > tc.width {
			t.Errorf("truncate(%q, %d) produced %d columns", tc.in, tc.width, w)
		}
	}
}

func TestLaneWidthsAreContentProportional(t *testing.T) {
	widths := laneWidths([]int{1, 2, 5, 3, 4}, 200)

	if widths[0] >= widths[2] {
		t.Errorf("a 1-card lane (%d) must be narrower than a 5-card lane (%d)", widths[0], widths[2])
	}
	if widths[2] != widths[3] {
		t.Errorf("lane weight is capped at 3, so 5 cards and 3 cards should match: %d vs %d",
			widths[2], widths[3])
	}

	total := laneGutter * 4
	for _, w := range widths {
		total += w
	}
	if total > 200 {
		t.Errorf("lanes total %d columns, more than the 200 available", total)
	}
}

func TestLaneWidthsRespectTheMinimum(t *testing.T) {
	widths := laneWidths([]int{1, 1, 1}, 120)
	for i, w := range widths {
		if w < minLaneWidth {
			t.Errorf("lane %d is %d columns, below the %d minimum", i, w, minLaneWidth)
		}
	}
}

func TestLaneWidthsSurviveTightBudgets(t *testing.T) {
	for _, width := range []int{60, 80, 100, 140, 300} {
		widths := laneWidths([]int{2, 2, 2, 2, 2, 2}, width)
		total := laneGutter * 5
		for _, w := range widths {
			total += w
			if w < 1 {
				t.Fatalf("at %d columns a lane came out %d wide", width, w)
			}
		}
		if total > width {
			t.Fatalf("at %d columns the lanes total %d", width, total)
		}
	}
}

func TestVisibleLanesFollowWidth(t *testing.T) {
	tests := []struct {
		width int
		lanes int
	}{
		{60, 2},
		{80, 2},
		{100, 3},
		{120, 4},
		{160, 5},
		{180, 6},
		{300, 10},
	}
	for _, tc := range tests {
		if got := (Layout{Width: tc.width, Height: 40}).MaxVisibleLanes(); got != tc.lanes {
			t.Errorf("%d cols fits %d lanes, want %d", tc.width, got, tc.lanes)
		}
	}
}

func TestSplitLeavesBothHalvesUsable(t *testing.T) {
	for _, body := range []int{8, 10, 20, 40, 60} {
		detail := (Layout{Width: 200, Height: body + 3}).SplitDetailHeight(body)
		if detail < 1 {
			t.Fatalf("body %d gave the detail pane %d rows", body, detail)
		}
		if board := body - detail; board < 1 {
			t.Fatalf("body %d left the board %d rows", body, board)
		}
	}
}

func TestLaneWindowKeepsSelectionVisible(t *testing.T) {
	for selected := 0; selected < 6; selected++ {
		offset, visible := laneWindow(6, selected, 3)
		if visible != 3 {
			t.Fatalf("selected %d: visible = %d, want %d", selected, visible, 3)
		}
		if selected < offset || selected >= offset+visible {
			t.Fatalf("selected lane %d is outside the window [%d,%d)", selected, offset, offset+visible)
		}
	}

	offset, visible := laneWindow(6, 3, 0)
	if offset != 0 || visible != 6 {
		t.Errorf("with no cap the window is everything, got offset %d visible %d", offset, visible)
	}
}
