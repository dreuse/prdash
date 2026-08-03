package ui

const (
	breakpointUnusable = 60
	breakpointShort    = 20

	laneGutter   = 3
	minLaneWidth = 26

	minSplitDetail = 6
	minSplitBoard  = 5
	numberGutter   = 7
)

type Layout struct {
	Width  int
	Height int
}

func (l Layout) TooSmall() bool { return l.Width < breakpointUnusable || l.Height < 6 }

func (l Layout) MaxVisibleLanes() int {
	return maxInt(1, (l.Width+laneGutter)/(minLaneWidth+laneGutter))
}

func (l Layout) ShowFilterBand() bool { return l.Height >= breakpointShort }

func (l Layout) SplitDetailHeight(body, want int) int {
	if body <= minSplitDetail+minSplitBoard {
		return maxInt(1, body-minSplitBoard)
	}
	if want <= 0 {
		want = body / 2
	}
	return clamp(want, minSplitDetail, body-minSplitBoard)
}

func laneWidths(counts []int, total int) []int {
	n := len(counts)
	widths := make([]int, n)
	if n == 0 {
		return widths
	}

	avail := total - laneGutter*(n-1)
	flexible := make([]int, 0, n)
	weights := make([]int, n)
	weightSum := 0

	for i, c := range counts {
		weights[i] = clamp(c, 1, 3)
		weightSum += weights[i]
		flexible = append(flexible, i)
	}

	if len(flexible) == 0 {
		return widths
	}

	base := minLaneWidth
	if avail < base*len(flexible) {
		base = maxInt(1, avail/len(flexible))
	}
	surplus := avail - base*len(flexible)
	if surplus < 0 {
		surplus = 0
	}

	used := 0
	for _, i := range flexible {
		widths[i] = base + surplus*weights[i]/weightSum
		used += widths[i]
	}
	if leftover := avail - used; leftover > 0 {
		widths[flexible[0]] += leftover
	}
	return widths
}

func laneWindow(count, selected, maxVisible int) (offset, visible int) {
	if maxVisible <= 0 || maxVisible >= count {
		return 0, count
	}
	visible = maxVisible
	offset = selected - visible/2
	offset = clamp(offset, 0, maxInt(0, count-visible))
	return offset, visible
}
