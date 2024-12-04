package main

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/montanaflynn/stats"
	exprand "golang.org/x/exp/rand"
	"golang.org/x/image/colornames"
	"gonum.org/v1/gonum/stat/distuv"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg/draw"
)

// func algoTABS_A(tabs float64, i int, tabValues []float64) float64 {
// 	f := tabValues[i]
//
// 	if f > tabs {
// 		consecutiveDrops = 0 // experimental
//
// 		tabs += tabs / 100
//
// 	} else if f < tabs {
// 		// // experimental
// 		// consecutiveDrops++
// 		// if consecutiveDrops > 9 {
// 		// 	consecutiveDrops = 9
// 		// }
// 		// tabs -= tabs / (100 / float64(consecutiveDrops))
//
// 		// experimental 2
// 		consecutiveDrops++
// 		consecutiveDrops = int(math.Min(float64(consecutiveDrops), float64(maxDrops))) // set ceiling of 9
//
// 		// floorDrops sets a floor for the drop rate multiplier
// 		floorDrops := consecutiveDrops
// 		if consecutiveDrops <= 42 {
// 			floorDrops = 1
// 		}
// 		tabs -= tabs / (100 / float64(floorDrops))
//
// 		// tabs -= tabs / (100)
// 	} else {
// 		tabs += 0
// 	}
// }

func TestTABSAdjustment1(t *testing.T) {

	dataSize := 1000000

	runExperiment := func(rander distuv.Rander, name string, valsOper func(int, []float64, func()) bool) {
		simulatedTABVals := []float64{}
		for i := 0; i < dataSize; i++ {
			r := rander.Rand()
			simulatedTABVals = append(simulatedTABVals, r)
		}

		// tabs := simulatedTABVals[rand.Intn(dataSize)]
		mean, _ := stats.Mean(simulatedTABVals)
		med, _ := stats.Median(simulatedTABVals)
		t.Logf("%s | mean: %v med: %v\n", name, mean, med)
		tabs := med

		tabsPlottable := plotter.XYs{}
		meansPlottable := plotter.XYs{}
		medsPlottable := plotter.XYs{}

		// consecutiveDrops := 0 // experimental
		// maxDrops := 20

		for i, f := range simulatedTABVals {

			// MODIFY TABS

			ratio := tabs / f           // eg. 0.84, 1.02, 1.58, 2.37
			ratio = math.Ceil(ratio)    // eg. 0, 1, 2, 3
			deltaNumerator := 1 - ratio // eg. 1, 0, -1, -2
			deltaDenominator := float64(2049)
			// set upper and lower bounds
			deltaNumerator = math.Min(deltaNumerator, 1)  // ceiling
			deltaNumerator = math.Max(deltaNumerator, -2) // floor

			adjustment := deltaNumerator / deltaDenominator
			tabs += tabs * adjustment

			// =>

			// ratio := f / tabs                       // eg. 0.981
			// ratioPerc := ratio * 100                // eg. 98.1
			// ratioPercRound := math.Floor(ratioPerc) // eg. 98
			// deltaNumerator := ratioPercRound - 100  // eg. -2
			// deltaDenominator := float64(2049)
			//
			// // set upper and lower bounds
			// deltaNumerator = math.Min(deltaNumerator, 1)   // ceiling
			// deltaNumerator = math.Max(deltaNumerator, -99) // floor
			//
			// adjustment := deltaNumerator / deltaDenominator
			// tabs += tabs * adjustment
			// => TABS drops off a cliff...

			// tabs = algoTABS_A(tabs, i, simulatedTABVals)

			// if f > tabs {
			// 	consecutiveDrops = 0 // experimental
			//
			// 	tabs += tabs / 100
			//
			// } else if f < tabs {
			// 	// // experimental
			// 	// consecutiveDrops++
			// 	// if consecutiveDrops > 9 {
			// 	// 	consecutiveDrops = 9
			// 	// }
			// 	// tabs -= tabs / (100 / float64(consecutiveDrops))
			//
			// 	// experimental 2
			// 	consecutiveDrops++
			// 	consecutiveDrops = int(math.Min(float64(consecutiveDrops), float64(maxDrops))) // set ceiling of 9
			//
			// 	// floorDrops sets a floor for the drop rate multiplier
			// 	floorDrops := consecutiveDrops
			// 	if consecutiveDrops <= 42 {
			// 		floorDrops = 1
			// 	}
			// 	tabs -= tabs / (100 / float64(floorDrops))
			//
			// 	// tabs -= tabs / (100)
			// } else {
			// 	tabs += 0
			// }

			tabsPlottable = append(tabsPlottable, plotter.XY{X: float64(i), Y: tabs})

			var modifiedVals bool
			if valsOper != nil {
				modifiedVals = valsOper(i, simulatedTABVals, func() {
					// Last reading before they
					mean, _ = stats.Mean(simulatedTABVals)
					meansPlottable = append(meansPlottable, plotter.XY{
						X: float64(i),
						Y: mean,
					})
					med, _ = stats.Median(simulatedTABVals)
					medsPlottable = append(medsPlottable, plotter.XY{
						X: float64(i),
						Y: med,
					})
				})
			}
			if modifiedVals || i == 0 || i == len(simulatedTABVals)-1 {
				mean, _ = stats.Mean(simulatedTABVals)
				meansPlottable = append(meansPlottable, plotter.XY{
					X: float64(i),
					Y: mean,
				})
				med, _ = stats.Median(simulatedTABVals)
				medsPlottable = append(medsPlottable, plotter.XY{
					X: float64(i),
					Y: med,
				})
			}

			// t.Log("tabs", tabs)
		}

		// sort.Float64s(simulatedTABVals)

		// for _, f := range simulatedTABVals {
		// 	t.Log(int(f))
		// }

		p, _ := plot.New()
		p.Title.Text = name
		p.Title.Padding = 16
		scatterTABS, _ := plotter.NewScatter(tabsPlottable)
		scatterTABS.Radius = 1
		scatterTABS.Shape = draw.CircleGlyph{}
		p.Add(scatterTABS)

		lineMean, _ := plotter.NewLine(meansPlottable)
		lineMean.Color = colornames.Green
		p.Add(lineMean)

		lineMed, _ := plotter.NewLine(medsPlottable)
		lineMed.Color = colornames.Blue
		p.Add(lineMed)

		chartFilepath := fmt.Sprintf("./%s.png", name)
		t.Log("Writing", chartFilepath)
		p.Save(800, 400, chartFilepath)
	}

	for _, d := range []struct {
		name     string
		dist     distuv.Rander
		valsOper func(int, []float64, func()) bool
	}{
		{
			"pareto",
			distuv.Pareto{
				Xm:    10,
				Alpha: 1,
				Src:   exprand.NewSource(uint64(time.Now().UnixNano())),
			},
			nil,
		},
		{
			"exponential",
			distuv.Exponential{
				Rate: 1,
				Src:  exprand.NewSource(uint64(time.Now().UnixNano())),
			},
			nil,
		},
		{
			"exponential_dynamic",
			distuv.Exponential{
				Rate: 1,
				Src:  exprand.NewSource(uint64(time.Now().UnixNano())),
			},
			// This operation drops all simuluated active balances by half.
			// Paper hands.
			func(i int, float64s []float64, beforeHook func()) bool {
				if i == len(float64s)/2 {
					beforeHook()
					for j, v := range float64s {
						float64s[j] = v / 2
					}
					return true
				}
				return false
			},
		},
		{
			"normal",
			distuv.Normal{
				Mu:    50,
				Sigma: math.Sqrt(1),
				Src:   exprand.NewSource(uint64(time.Now().UnixNano())),
			},
			nil,
		},
	} {
		runExperiment(d.dist, d.name, d.valsOper)
	}
}
