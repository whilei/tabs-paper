package main

import (
	"sort"
)

type HashrateDistType int

const (
	HashrateDistEqual HashrateDistType = iota
	HashrateDistLongtail
)

func (t HashrateDistType) String() string {
	switch t {
	case HashrateDistEqual:
		return "equal"
	case HashrateDistLongtail:
		return "longtail"
	default:
		panic("unknown")
	}
}

func generateMinerHashrates(ty HashrateDistType, n int) []float64 {
	if n < 1 {
		panic("must have at least one miner")
	}
	if n == 1 {
		return []float64{1}
	}

	out := []float64{}

	switch ty {
	case HashrateDistLongtail:
		rem := float64(1)
		for i := 0; i < n; i++ {
			var take float64
			var share float64
			if i == 0 {
				share = float64(1) / 3
			} else {
				share = 0.6
			}
			if i != n-1 {
				take = rem * share
			}
			if take > float64(1)/3*rem {
				take = float64(1) / 3 * rem
			}
			if i == n-1 {
				take = rem
			}
			out = append(out, take)
			rem = rem - take
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i] > out[j]
		})
		return out
	case HashrateDistEqual:
		for i := 0; i < n; i++ {
			out = append(out, float64(1)/float64(n))
		}
		return out
	default:
		panic("impossible")
	}
}
