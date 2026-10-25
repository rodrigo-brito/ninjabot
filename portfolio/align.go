package portfolio

import (
	"sort"

	"github.com/rodrigo-brito/ninjabot/backtest"
)

// AlignForwardFill puts several curves on the union of their timestamps.
// At a time without a point, a curve keeps its last value (forward fill);
// before its first point it is worth its initial value. The result lists
// the times, sorted, and one value per time for each curve.
func AlignForwardFill(curves map[string][]backtest.Point, initial map[string]float64) ([]int64, map[string][]float64) {
	unique := make(map[int64]struct{})
	for _, curve := range curves {
		for _, point := range curve {
			unique[point.Time] = struct{}{}
		}
	}
	times := make([]int64, 0, len(unique))
	for t := range unique {
		times = append(times, t)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })

	aligned := make(map[string][]float64, len(curves))
	for id, curve := range curves {
		values := make([]float64, len(times))
		value := initial[id]
		next := 0
		for i, t := range times {
			for next < len(curve) && curve[next].Time <= t {
				value = curve[next].Value
				next++
			}
			values[i] = value
		}
		aligned[id] = values
	}
	return times, aligned
}

// sample keeps the points of a curve at the times of a coarser grid: every
// time that is a multiple of step seconds.
func sample(times []int64, values []float64, step int64) []float64 {
	if step <= 0 {
		return values
	}
	out := make([]float64, 0, len(values))
	for i, t := range times {
		if t%step == 0 {
			out = append(out, values[i])
		}
	}
	return out
}

// returns turns levels into the relative changes between them.
func returns(values []float64) []float64 {
	out := make([]float64, 0, len(values))
	for i := 1; i < len(values); i++ {
		if values[i-1] != 0 {
			out = append(out, (values[i]-values[i-1])/values[i-1])
		} else {
			out = append(out, 0)
		}
	}
	return out
}
