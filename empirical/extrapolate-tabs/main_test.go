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

func TestTABSAdjustment1(t *testing.T) {

	dataSize := 1000000

	runExperiment := func(rander distuv.Rander, name string) {
		simulatedTABVals := []float64{}
		for i := 0; i < dataSize; i++ {
			r := rander.Rand()
			simulatedTABVals = append(simulatedTABVals, r)
		}

		mean, _ := stats.Mean(simulatedTABVals)
		med, _ := stats.Median(simulatedTABVals)
		t.Logf("%s | mean: %v med: %v\n", name, mean, med)

		// tabs := simulatedTABVals[rand.Intn(dataSize)]
		tabs := med

		tabsPlottable := plotter.XYs{}

		consecutiveDrops := 0 // experimental
		maxDrops := 20
		for i, f := range simulatedTABVals {
			if f > tabs {
				consecutiveDrops = 0 // experimental

				tabs += tabs / 100

			} else if f < tabs {
				// // experimental
				// consecutiveDrops++
				// if consecutiveDrops > 9 {
				// 	consecutiveDrops = 9
				// }
				// tabs -= tabs / (100 / float64(consecutiveDrops))

				// experimental 2
				consecutiveDrops++
				consecutiveDrops = int(math.Min(float64(consecutiveDrops), float64(maxDrops))) // set ceiling of 9

				// floorDrops sets a floor for the drop rate multiplier
				floorDrops := consecutiveDrops
				if consecutiveDrops <= 10 {
					floorDrops = 1
				}
				tabs -= tabs / (100 / float64(floorDrops))

				// tabs -= tabs / (100)
			} else {
				tabs += 0
			}
			tabsPlottable = append(tabsPlottable, plotter.XY{X: float64(i), Y: tabs})
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

		lineMean, _ := plotter.NewLine(plotter.XYs{
			plotter.XY{0, mean},
			plotter.XY{float64(dataSize), mean},
		})
		lineMean.Color = colornames.Green
		p.Add(lineMean)

		lineMed, _ := plotter.NewLine(plotter.XYs{
			plotter.XY{0, med},
			plotter.XY{float64(dataSize), med},
		})
		lineMed.Color = colornames.Blue
		p.Add(lineMed)

		p.Save(800, 400, fmt.Sprintf("./%s.png", name))
	}

	for _, d := range []struct {
		name string
		dist distuv.Rander
	}{
		{
			"pareto",
			distuv.Pareto{
				Xm:    10,
				Alpha: 1,
				Src:   exprand.NewSource(uint64(time.Now().UnixNano())),
			},
		},
		{
			"exponential",
			distuv.Exponential{
				Rate: 1,
				Src:  exprand.NewSource(uint64(time.Now().UnixNano())),
			},
		},
		{
			"normal",
			distuv.Normal{
				Mu:    50,
				Sigma: math.Sqrt(1),
				Src:   exprand.NewSource(uint64(time.Now().UnixNano())),
			},
		},
	} {
		runExperiment(d.dist, d.name)
	}
}
