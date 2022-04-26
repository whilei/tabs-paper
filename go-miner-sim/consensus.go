package main

// detBlockDifficulty calculates difficulty.
// interval should be factored by tick rate already.
func detBlockDifficulty(parent *Block, uncles bool, interval int64) int64 {
	x := interval / (9) // 9 SECONDS
	y := 1 - x
	if uncles {
		y = 2 - x
	}
	if y < -99 {
		y = -99
	}
	return int64(float64(parent.d) + (float64(y) / 2048 * float64(parent.d)))
}

func getTABS(parent *Block, localTAB int64) int64 {
	scalarNumerator := int64(0)
	if localTAB > parent.tabs {
		scalarNumerator = 1
	} else if localTAB < parent.tabs {
		scalarNumerator = -1
	}

	numerator := tabsAdjustmentDenominator + scalarNumerator // [127|128|129]/128, [4095|4096|4097]/4096

	return int64(float64(parent.tabs) * float64(numerator) / float64(tabsAdjustmentDenominator))
}

type ConsensusAlgorithm int

const (
	None ConsensusAlgorithm = iota
	TD
	TDTABS
	TimeAsc
	TimeDesc // FreshnessPreferred
)

func (c ConsensusAlgorithm) String() string {
	switch c {
	case TD:
		return "TD"
	case TDTABS:
		return "TDTABS"
	case TimeAsc:
		return "TimeAsc"
	case TimeDesc:
		return "TimeDesc"
	}
	panic("impossible")
}
