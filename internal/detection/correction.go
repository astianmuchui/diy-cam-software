package detection

import (
	"math"
	"sort"
	"sync"
)

type DepthTiltPair struct {
	Depth float64
	Tilt  float64
}

type TiltCorrector struct {
	mu        sync.Mutex
	window    []DepthTiltPair
	maxSize   int
	slope     float64
	intercept float64
	rSquared  float64
	baseline  float64
}

func NewTiltCorrector(windowSize int) *TiltCorrector {
	if windowSize <= 0 {
		windowSize = 50
	}
	return &TiltCorrector{maxSize: windowSize}
}

func (tc *TiltCorrector) Correct(depth, tilt float64) (correctedDepth, baseline float64) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.window = append(tc.window, DepthTiltPair{Depth: depth, Tilt: tilt})
	if len(tc.window) > tc.maxSize {
		tc.window = tc.window[1:]
	}

	if len(tc.window) < 3 {
		return depth, depth
	}

	tc.recompute()

	predicted := tc.slope*tilt + tc.intercept
	correctedDepth = tc.intercept + (depth - predicted)
	return correctedDepth, tc.baseline
}

func (tc *TiltCorrector) Params() (slope, intercept, rSquared, baseline float64, windowLen int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.slope, tc.intercept, tc.rSquared, tc.baseline, len(tc.window)
}

func (tc *TiltCorrector) recompute() {
	n := float64(len(tc.window))
	var sumX, sumY, sumXY, sumX2 float64
	for _, p := range tc.window {
		sumX += p.Tilt
		sumY += p.Depth
		sumXY += p.Tilt * p.Depth
		sumX2 += p.Tilt * p.Tilt
	}
	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-9 {

		return
	}
	tc.slope = (n*sumXY - sumX*sumY) / denom
	tc.intercept = (sumY - tc.slope*sumX) / n

	meanY := sumY / n
	var ssTot, ssRes float64
	correctedDepths := make([]float64, len(tc.window))
	for i, p := range tc.window {
		predicted := tc.slope*p.Tilt + tc.intercept
		corrected := tc.intercept + (p.Depth - predicted)
		correctedDepths[i] = corrected
		residual := p.Depth - predicted
		ssRes += residual * residual
		diff := p.Depth - meanY
		ssTot += diff * diff
	}
	if ssTot > 0 {
		tc.rSquared = 1 - ssRes/ssTot
	}

	sorted := make([]float64, len(correctedDepths))
	copy(sorted, correctedDepths)
	sort.Float64s(sorted)
	cutoff := len(sorted) - len(sorted)*30/100
	if cutoff >= len(sorted) {
		cutoff = len(sorted) - 1
	}
	deepSlice := sorted[cutoff:]
	var sum float64
	for _, d := range deepSlice {
		sum += d
	}
	tc.baseline = sum / float64(len(deepSlice))
}
