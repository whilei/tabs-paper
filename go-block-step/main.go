/*
Instead of math, use calculator.

If we assume a Poisson Point Process distribution for the independent miners, what is the probability expected for 2 independent new-block events to occur within an interval $\eta$ of each other?
*/

package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/montanaflynn/stats"
	exprand "golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg/draw"
)

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

var tickMultiple = 1

type RoundConfiguration struct {
	Name               string
	NetworkLambda      float64
	Latency            float64
	SelfishDelay       float64
	TickMultiple       int
	Rounds             int // aka Blocks
	NumberOfMiners     int
	HashrateDistType   HashrateDistType
	ConsensusAlgorithm ConsensusAlgorithm
}

func (p RoundConfiguration) String() string {
	return fmt.Sprintf(`

	Name:             %s, ConsensusAlgorithm: %s,
	NetworkLambda:    %d, Latency:          %0.2f,
	TickMultiple:     %d, Rounds:           %d,
	NumberOfMiners:   %d, HashrateDistType: %s,
`,
		p.Name, p.ConsensusAlgorithm,
		int(p.NetworkLambda), p.Latency,
		p.TickMultiple, p.Rounds,
		p.NumberOfMiners, p.HashrateDistType,
	)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	for pIndex, config := range []RoundConfiguration{
		// {
		// 	// Pretty realistic.
		// 	Name: "A", ConsensusAlgorithm: TD,
		// 	NetworkLambda: 14, Latency: 1.9, // NOTE: Latency 1.9 vs 2.0 seems bimodal. This makes sense because... rounding?
		// 	TickMultiple: 1, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },
		// {
		// 	// Exploring latency binomialism.
		// 	Name: "A", ConsensusAlgorithm: TD,
		// 	NetworkLambda: 14, Latency: 2.1, // NOTE: Latency 1.9 vs 2.0 seems bimodal. This makes sense because... rounding?
		// 	TickMultiple: 1, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },
		{
			// Pretty realistic.
			Name: "A", ConsensusAlgorithm: TD,
			NetworkLambda: 14, Latency: 2.4, SelfishDelay: 1, // 1.4 ,
			TickMultiple: 10, Rounds: 10000,
			NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		},
		// {
		// 	// Pretty realistic.
		// 	Name: "A", ConsensusAlgorithm: TDTABS,
		// 	NetworkLambda: 14, Latency: 1.4, // 1.4 ,
		// 	TickMultiple: 100, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },
		// {
		// 	// Exploring latency binomialism.
		// 	Name: "A", ConsensusAlgorithm: TD,
		// 	NetworkLambda: 14, Latency: 2.1, // NOTE: Latency 1.9 vs 2.0 seems bimodal. This makes sense because... rounding?
		// 	TickMultiple: 10, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },
		// {
		// 	Name: "A", ConsensusAlgorithm: TD,
		// 	NetworkLambda: 14, Latency: 2.8,
		// 	TickMultiple: 1, Rounds: 30000, // many rounds
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },
		// {
		// 	// An initial comparison to TDTABS.
		// 	Name: "A", ConsensusAlgorithm: TDTABS,
		// 	NetworkLambda: 14, Latency: 2.8,
		// 	TickMultiple: 1, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },
		// {
		// 	// Show that equally-distributed capitals (hashrate and balances) cause TDTABS to be invariant.
		// 	Name: "B", ConsensusAlgorithm: TD,
		// 	NetworkLambda: 14, Latency: 2.8,
		// 	TickMultiple: 1, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistEqual, // equally distributed hashrates (and by proxy, balances)
		// },
		// {
		// 	Name: "B", ConsensusAlgorithm: TDTABS,
		// 	NetworkLambda: 14, Latency: 2.8,
		// 	TickMultiple: 1, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistEqual, // equally distributed hashrates (and by proxy, balances)
		// },
		// {
		// 	// Deeper dive into how the test works. Get a sense for how time is modeled and what is (and is not) assumed.
		// 	Name: "C", ConsensusAlgorithm: TDTABS,
		// 	NetworkLambda: 14, Latency: 0, // no latency
		// 	TickMultiple: 1, Rounds: 10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },
		// {
		// 	Name: "C", ConsensusAlgorithm: TDTABS,
		// 	NetworkLambda: 14, Latency: 0, // no latency
		// 	TickMultiple:   100, // greater tick interval
		// 	Rounds:         10000,
		// 	NumberOfMiners: 8, HashrateDistType: HashrateDistLongtail,
		// },

		// Shorter latency and slightly lower lambda, still pretty realistic, maybe.
		// Use greater TickMultiple values, taking longer to run, but with a "more realistic"/granular
		// model of passage of time.
		// The greater the TickMultiple value, the longer the program will take to run.
		// {
		// 	NetworkLambda:    13,
		// 	Latency:          1.5,
		// 	TickMultiple:     1000,
		// 	Rounds:           10000,
		// 	NumberOfMiners:   8,
		// 	HashrateDistType: HashrateDistLongtail,
		// },
		// {
		// 	NetworkLambda:    13,
		// 	Latency:          1.5,
		// 	// TickMultiple:     1000,
		// 	Rounds:           10000,
		// 	NumberOfMiners:   8,
		// 	HashrateDistType: HashrateDistEqual, // equally distributed hashrates (and by proxy, balances)
		// },
	} {

		start := time.Now()

		// Usage of the 1000 value suggests a time unit of milliseconds.
		tickMultiple = config.TickMultiple // Modify the global for now. Ugly but whatever. Won't run in parallel.

		networkLambdaTicks := config.NetworkLambda * float64(tickMultiple) // ie. 13*1000ms = 13s
		latencyTicks := config.Latency * float64(tickMultiple)             // eg. 350ms

		logger := log.New(os.Stdout, "", 0)

		logger.Println("-----------------------------------------------------")

		logger.Println("CONFIG", config)

		minersHashrates := generateMinerHashrates(config.HashrateDistType, config.NumberOfMiners)

		minerHashrateChecksum := float64(0)
		for _, mhr := range minersHashrates {
			minerHashrateChecksum += mhr
		}
		printHashrates := func() string {
			out := "["
			for _, m := range minersHashrates {
				out += fmt.Sprintf("%0.3f ", m)
			}
			out = out[:len(out)-2]
			out += "]"
			return out
		}()

		// Print generated miner entities.
		logger.Printf(`GENERATED MINERS

	Number: %d, Distribution: %s, Hashrate Checksum OK:    %v,
	Hashrates: %s

`, len(minersHashrates), config.HashrateDistType, minerHashrateChecksum > 0.999, printHashrates)

		// miners by-hashrate wins
		minerWinTicks := make([][]float64, len(minersHashrates))
		minerWinIntervals := make([][]float64, len(minersHashrates))

		// Tallies and state variables.
		recordedSubjectiveWinnerIntervals := []float64{}
		originalNCandidatesRound := []float64{}
		canonicalMinerIndexes := []int{} // the indexes (by minerHashrates data) of canonical-winning miners

		totalTicks := 0

		sameMinerIntervals := []float64{}
		lastWinnerIndex := -1
		solverSameTally := 0 // when the previous round is won by the same author

		arbitrationDecisiveTally := 0
		arbitrationIndecisiveTally := 0

		// Let's get a Poisson model in there just for visual comparison.
		exprand.Seed(uint64(time.Now().UnixNano()))
		poisson := distuv.Poisson{
			Lambda: 1 / networkLambdaTicks,
			Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
		}
		lastPoissonNonZeroTick := totalTicks // poisson tick start time
		poissonIntervals := []float64{}

		for i := 1; i <= config.Rounds; i++ {

			// [0,2,3], [484,525]
			// Associated author hashrates: minerHashrates[authorIndexes[i]]
			authorIndexes, tooks := hashrateRace(minersHashrates, -1, networkLambdaTicks) // guaranteed 1 result

			// We now have the "objective" winners.

			// Next, we collect potential winners from the subsequent latency period.

			// We need a list of miners to not include in the latents-only hashrate race.
			// These are only miners who have not yet found a solution and get to continue mining in the latency interval.
			latentHashrates := []float64{}

			for jj, hr := range minersHashrates {
				hit := false
				for _, ai := range authorIndexes {
					if jj == ai {
						hit = true
					}
				}
				if !hit {
					latentHashrates = append(latentHashrates, hr)
				}
			}

			// Finally, have the latent authors continue their race for the latency period ticks.
			latentAuthors, latentTooks := hashrateRace(latentHashrates, int(latencyTicks), networkLambdaTicks)

			// Append any latent winners to the network-level winners/intervals pool.
			authorIndexes = append(authorIndexes, latentAuthors...)
			tooks = append(tooks, latentTooks...)

			// We can now tally the total number of eligible authors.
			originalNCandidatesRound = append(originalNCandidatesRound, float64(len(authorIndexes)))

			// Since we're guaranteed 1 result, we set the first to default.
			// Which (index) of the []authorIndexes is the winner.
			winnerIndex := 0

			// ARBITRATION.
			// Handle cases with multiple solution candidates.
			if len(tooks) > 1 {
				switch config.ConsensusAlgorithm {
				case TD:
					winnerIndex = decideTD(tooks)
				case TDTABS:
					// We have to pass minersHashrates and authorIndexes because the TABS part
					// needs to know the candidate miners hashrates, which it currently uses to
					// approximate available active balance (in Wei).
					winnerIndex = decideTDTABS(minersHashrates, authorIndexes, tooks)
				}

				if winnerIndex != -1 {
					arbitrationDecisiveTally++
				} else {
					arbitrationIndecisiveTally++
				}
			}

			// The objective arbitration was indecisive.
			if winnerIndex == -1 {
				// Flip a coin.
				winnerIndex = rand.Intn(len(tooks))
			}

			winTook := tooks[winnerIndex]

			// An eligible block has been found and broadcast.

			// Record this interval as the winner.

			// We add the latency value to the measurement of the interval since
			// block intervals are measured with timestamps, so we expect the distance in timestamps
			// between two blocks to include the latency interval (IF THE MINERS ARE DIFFERENT).
			// We assume everyone uses NIST.
			recordedInterval := float64(winTook)

			// We do not assume that the miners are different. Only add the latency interval
			// when the miner is different that of its successive round.
			sameParentSolver := lastWinnerIndex == authorIndexes[winnerIndex]
			if sameParentSolver {
				solverSameTally++
			} else {
				delay := latencyTicks
				if config.SelfishDelay > 0 && minersHashrates[authorIndexes[winnerIndex]] > 0.25 {
					delay +=config.SelfishDelay
				}
				recordedInterval += delay
			}
			lastWinnerIndex = authorIndexes[winnerIndex] // housekeeping

			recordedSubjectiveWinnerIntervals = append(recordedSubjectiveWinnerIntervals, recordedInterval)
			canonicalMinerIndexes = append(canonicalMinerIndexes, authorIndexes[winnerIndex])

			if sameParentSolver {
				sameMinerIntervals = append(sameMinerIntervals, recordedInterval)
			}

			minerWinTicks[authorIndexes[winnerIndex]] = append(minerWinTicks[authorIndexes[winnerIndex]], float64(totalTicks))
			minerWinIntervals[authorIndexes[winnerIndex]] = append(minerWinIntervals[authorIndexes[winnerIndex]], float64(recordedInterval))

			// POISSON overlay:

			// The following takes random Poisson distribution samples
			// for the duration of the winning interval.
			tMax := totalTicks + 1*winTook // + 1 * tickMultiple
		poissonSampleLoop:
			for t := totalTicks; t < tMax; t++ {
				k := poisson.Rand()
				kInt := int(k)
				if kInt == 0 {
					continue poissonSampleLoop
				}
				// k is positive
				gotInterval := float64(t) - float64(lastPoissonNonZeroTick)
				if !sameParentSolver {
					gotInterval += latencyTicks
				}
				poissonIntervals = append(poissonIntervals, gotInterval)
				lastPoissonNonZeroTick = t
			}

			// Total ticks is summed now because (so far) we're only measuring
			// the objective winner interval.
			totalTicks += winTook
		}

		logger.Println(printStats("INTERVALS", recordedSubjectiveWinnerIntervals))
		logger.Println(printStats("ELIGIBLE AUTHORS PER BLOCK", originalNCandidatesRound))

		logger.Println("MINER CANONICAL WINS")
		logger.Println()
		for i, m := range minersHashrates {
			sum := 0
			for _, c := range canonicalMinerIndexes {
				if c == i {
					sum++
				}
			}
			winrate := float64(sum) / float64(config.Rounds)
			logger.Printf(`	miner=%d hashrate=%0.3f winrate=%0.3f winrate/hashrate=%0.3f (%d)`,
				i, m, winrate, winrate/m, sum)
		}
		logger.Println()

		logger.Printf(`ANALYSIS

	Ticks: %d, Rounds (Blocks): %d, Ticks/Block: %v
	AuthorSameParentChildTally/Block: %0.3f
	ArbitrationDecisiveRate: %0.3f, ArbitrationDecisiveTally: %d
	ArbitrationIndecisiveRate: %0.3f, ArbitrationIndecisiveTally: %d

`,
			totalTicks, config.Rounds, float64(totalTicks)/float64(config.Rounds),
			float64(solverSameTally)/float64(config.Rounds),
			float64(arbitrationDecisiveTally)/float64(config.Rounds),
			arbitrationDecisiveTally,
			float64(arbitrationIndecisiveTally)/float64(config.Rounds),
			arbitrationIndecisiveTally,
		)

		// META logger
		logger.Printf(`
	Elapsed: %v
`,
			time.Since(start).Round(time.Millisecond),
		)

		dir := filepath.Join(os.TempDir(), "plots", "hfctabs",
			fmt.Sprintf("%s%d", config.Name, pIndex))
		os.MkdirAll(dir, os.ModePerm)

		filename := fmt.Sprintf("canonicalBlockIntervals.png")

		// Put intervals in buckets and in a histogram.
		// Do they look Poisson-y?
		p := plot.New()

		buckets := map[int]int{}
		for _, v := range recordedSubjectiveWinnerIntervals {
			vInt := int(v)
			bucket := vInt / tickMultiple // this value will be floored
			buckets[bucket]++
		}

		data := plotter.XYs{}
		for k, v := range buckets {
			data = append(data, plotter.XY{X: float64(k), Y: float64(v)})
		}
		hist, err := plotter.NewHistogram(data, len(buckets))
		if err != nil {
			panic(err)
		}
		p.Add(hist)
		p.Legend.Add("Canonical intervals", hist)

		p.Title.Text = fmt.Sprintf("Modeled Block Intervals (miners=%d[dist=%s], blocks=%d, lambda=%d, latency=%0.1f)",
			config.NumberOfMiners, config.HashrateDistType,
			config.Rounds, int(networkLambdaTicks), latencyTicks)
		p.Title.Padding = 16
		p.Legend.Top = true
		p.Legend.Padding = 8
		p.Legend.YOffs = -16
		p.Legend.XOffs = -16
		p.X.Label.Text = "Canonical Block Intervals"
		p.X.Min = 0
		p.Y.Label.Text = "Occurrences"

		// Same miner interval histogram
		buckets = map[int]int{}
		for _, v := range sameMinerIntervals {
			bucket := int(v) / tickMultiple
			buckets[bucket]++
		}
		data = plotter.XYs{}
		for k, v := range buckets {
			data = append(data, plotter.XY{X: float64(k), Y: float64(v)})
		}
		hist, _ = plotter.NewHistogram(data, len(buckets))
		hist.FillColor = color.RGBA{R: 100, G: 100, B: 255, A: 255}
		p.Add(hist)
		p.Legend.Add("Same miner intervals", hist)
		// End same miner interval histogram

		// Poisson overlay
		// p = plot.New()
		buckets = map[int]int{}
		for _, v := range poissonIntervals {
			bucket := int(v) / tickMultiple
			buckets[bucket]++
		}
		data = plotter.XYs{}
		for k, v := range buckets {
			data = append(data, plotter.XY{X: float64(k), Y: float64(v)})
		}
		scatter, _ := plotter.NewScatter(data)
		scatter.Radius = 2
		scatter.Shape = draw.CircleGlyph{}
		scatter.Color = color.RGBA{R: 255, G: 100, B: 100, A: 255}
		p.Add(scatter)
		p.Legend.Add("Poisson", scatter)

		// logger.Println()
		// logger.Println(len(poissonIntervals), printStats("POISSON INTERVALS", poissonIntervals))
		// End Poisson overlay

		if err := p.Save(800, 300, filepath.Join(dir, filename)); err != nil {
			panic(err)
		}

	}

}

func printStats(name string, data []float64) string {
	mean, _ := stats.Mean(data)
	med, _ := stats.Median(data)
	mode, _ := stats.Mode(data)
	min, _ := stats.Min(data)
	max, _ := stats.Max(data)
	return fmt.Sprintf(`%s
 
	Mean: %0.4f,
	Med: %0.4f, Mode: %0.4f,
	Min: %0.4f, Max: %0.4f,

`,
		name,
		mean, med, mode, min, max,
	)
}

// generateMinerHashrates sets up a set of miner annotated by a dynamic, respective hashrates.
/*
	eg.
	Miners=8
	[0.33 0.33499999999999996 0.16749999999999998 0.08374999999999999
	0.041874999999999996 0.020937499999999998 0.010468749999999999 0.010468749999999999]
	checksum=1.00
*/

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

// hashrateRace returns the index values of the hashrates that "found" solutions to a made up puzzle.
// Their respective elapsed times (in ticks) is returned in the second position.
// A maxTicks value of -1 causes the function to produce at least one winner in an arbitrary amount of time. Consider yourself warned.
// The lambda value controls ~something like the average~ value this function ~should~ return for a 'took' value.
/*
Simulated Guessing (Haystack-Independent model).

A simulated miner "attempts" to find a random "needle in a haystack," simulating the search for PoW puzzle solutions.
The miner's efficacy is determined by their HPS over some arbitrary unit of time.
Note that this allows the model to use rational numbers, which gives it a greater granularity than Poisson (which is integer based),
ie. time units less than 1 second can be explored.
*/
func hashrateRace(hashrates []float64, maxTicks int, lambda float64) (authorIndexes []int, tooks []int) {

	/*
		This algorithm implements rounds as drawn below,
		which each round representing 1 second of search time.
		Rounds are independent: the miner can re-search a space on the interval
		since ethash rounds are independent.
		And because Ethash has an indefinite problem space, and the random needle/haystack here has known bounds.

		MISS
		-----------------|-------
		--__x__------------------

		MISS
		-----------------|-------
		x__--------------------__

		HIT
		-----------------|-------
		--------------__x__------
	*/
	needle := rand.Float64()

	for elapsedTicks := 1; elapsedTicks < maxTicks || (maxTicks == -1 && len(authorIndexes) == 0); elapsedTicks++ {
		for i, hr := range hashrates {
			trial := rand.Float64()

			// The miner's share of the network's hashrate, per tick.
			tickR := hr * (1 /*tick*/ / lambda)

			// Divide by two because using absolute value (math only needs on half of the window).
			tickR = tickR / 2

			// This miner found a solution.
			if math.Abs(trial-needle) <= tickR ||
				math.Abs(trial-needle) >= 1-tickR {

				authorIndexes = append(authorIndexes, i)
				tooks = append(tooks, elapsedTicks)

				// Do not 'break' or return here; we want to allow multiple solvers in same tick.
			}
		}
	}
	return
}

func getTD(elapse int) float64 {
	x := (elapse / (9 * tickMultiple)) // int
	y := 1 - x
	if y < -99 {
		y = -99
	}
	return 1 + (float64(y) / 2048)
}

// decideTD returns the index of the winner using bucketed intervals (per Ethereum Difficulty algo)
// or -1 if it was undecided.
func decideTD(tickElapses []int) (winnerIndex int) {
	// Set default as undecided.
	winnerIndex = -1

	winningTD := float64(0)

	for i, v := range tickElapses {
		td := getTD(v)
		if td > winningTD {
			winningTD = td
			winnerIndex = i
		}
	}

	// We now have a greater Bucket value,
	// but we have not conclusively determined (whether there was only) a single winner.
	winnerTally := 0
	for _, v := range tickElapses {
		td := getTD(v)
		if td == winningTD {
			winnerTally++
		}
	}
	if winnerTally == 1 {
		return winnerIndex
	}
	return -1
}

// decideTime is an experiment.
func decideTime(tickElapses []int) (winnerIndex int) {
	winnerIndex = -1
	winningElapse := math.MaxInt16
	for i, v := range tickElapses {
		if v <= winningElapse {
			winnerIndex = i
			winningElapse = v
		}
	}

	// We now have the best (smallest) time value,
	// but we have not conclusively determined (whether there was only) a single winner.
	winnerTally := 0
	for _, v := range tickElapses {
		if v == winningElapse {
			winnerTally++
		}
	}
	if winnerTally == 1 {
		return winnerIndex
	}
	return -1
}

func getTABS(referenceTABS float64, balance float64) float64 {
	tabs := float64(1)
	if balance < referenceTABS {
		tabs = float64(127) / float64(128)
	} else if balance > referenceTABS {
		tabs = float64(129) / float64(128)
	}
	return tabs
}

// decideTDTABS is a naive estimation of TDTABS arbitration.
// It will grow to be a sophisticated model, but for now its going to make some assumptions.
// We'll assume that hashrate proportions ARE block capital proportions.
// This implicitly assumes that TAB measurements contributed by transactions are a constant for all miners.
//
func decideTDTABS(minerHashrates []float64, authorIndexes []int, tickElapses []int) (winnerIndex int) {
	// Set default as undecided.
	winnerIndex = -1

	winningTDTABS := float64(0)

	// Let's use a med balance and improve our model of TABS
	medianBalance, _ := stats.Median(minerHashrates)

	for i, v := range tickElapses {
		td := getTD(v)
		hashrate := minerHashrates[authorIndexes[i]]
		balance := hashrate // Assumed, for now.

		tdtabs := td * getTABS(medianBalance, balance)

		if tdtabs > winningTDTABS {
			winningTDTABS = tdtabs
			winnerIndex = i
		}
	}

	// We now have a greater Bucket value,
	// but we have not conclusively determined (whether there was only) a single winner.
	winnerTally := 0
	for i, v := range tickElapses {
		td := getTD(v)
		hashrate := minerHashrates[authorIndexes[i]]
		balance := hashrate // Assumed, for now.

		tdtabs := td * getTABS(medianBalance, balance)

		if tdtabs == winningTDTABS {
			winnerTally++
		}
	}
	if winnerTally == 1 {
		return winnerIndex
	}
	return -1
}
