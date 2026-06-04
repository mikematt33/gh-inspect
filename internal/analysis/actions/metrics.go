package actions

import (
	"math"
	"sort"
	"time"
)

// mean returns the arithmetic mean of the values, or 0 for an empty slice.
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// maxOf returns the maximum value, or 0 for an empty slice.
func maxOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// percentile returns the p-th percentile (0-100) of values using linear
// interpolation. Values do not need to be pre-sorted.
func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	rank := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// median returns the 50th percentile of the values.
func median(values []float64) float64 {
	return percentile(values, 50)
}

// flakiness scores how often consecutive runs alternate between success and
// failure. Conclusions must be ordered chronologically (oldest first). The
// result is the fraction of adjacent pass/fail transitions (0 = stable,
// 1 = alternates every run). Non success/failure conclusions are ignored.
func flakiness(conclusionsChrono []string) float64 {
	var states []bool // true = success, false = failure
	for _, c := range conclusionsChrono {
		switch c {
		case "success":
			states = append(states, true)
		case "failure", "timed_out", "startup_failure":
			states = append(states, false)
		}
	}
	if len(states) < 2 {
		return 0
	}
	transitions := 0
	for i := 1; i < len(states); i++ {
		if states[i] != states[i-1] {
			transitions++
		}
	}
	return float64(transitions) / float64(len(states)-1)
}

// mtbfHours computes the mean time between failures in hours given failure
// timestamps. With fewer than two failures it returns 0 (undefined).
func mtbfHours(failureTimes []time.Time) float64 {
	if len(failureTimes) < 2 {
		return 0
	}
	sorted := make([]time.Time, len(failureTimes))
	copy(sorted, failureTimes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	span := sorted[len(sorted)-1].Sub(sorted[0])
	gaps := len(sorted) - 1
	return span.Hours() / float64(gaps)
}

// durationTrendSec compares the average duration of the most recent half of
// runs against the earliest half. Durations must be ordered chronologically
// (oldest first). A positive result means the workflow is trending slower.
func durationTrendSec(durationsChrono []float64) float64 {
	n := len(durationsChrono)
	if n < 4 {
		return 0
	}
	half := n / 2
	earlier := mean(durationsChrono[:half])
	later := mean(durationsChrono[half:])
	return later - earlier
}
