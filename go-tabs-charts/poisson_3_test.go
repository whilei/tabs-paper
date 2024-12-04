package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	exprand "golang.org/x/exp/rand"
	"golang.org/x/exp/shiny/materialdesign/colornames"
	"gonum.org/v1/gonum/stat/distuv"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestChartPoissonLambdas3(t *testing.T) {
	// 1/14, 1/14/2, 1/14/4, 1/14/6, 1/14/8, 1/14/16
	var networkLambda float64 = float64(1) / float64(13)
	var hashrateProportions = []float64{0.25}
	var intervalSamples float64 = 99

	p, _ := plot.New()
	p.Title.Text = "Adjusted Poisson Lambda vs. Adjusted Poisson Gross"

	p.X.Label.Text = "Time"
	p.X.Min = 0
	p.X.Max = intervalSamples
	p.X.Tick.Marker = DifficultyModTickerInterval{}

	p.Y.Label.Text = "Probability"
	p.Y.Min = 0
	p.Y.Max = 1

	var data = plotter.XYs{}
	var data2 = plotter.XYs{}

	for _, hashratePortion := range hashrateProportions {
		for interval := float64(0); interval <= intervalSamples; interval++ {
			poissonDist := distuv.Poisson{
				Lambda: interval * networkLambda * hashratePortion,
				Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
			}
			y := 1 - poissonDist.CDF(0)
			data = append(data, plotter.XY{X: interval, Y: y})

			data2 = append(data2, plotter.XY{X: interval, Y: 4 * y})
		}

		line, _ := plotter.NewLine(data)
		line.Color = colornames.Green200
		p.Add(line)
		p.Legend.Add(fmt.Sprintf("1 - (Poisson(1/13 * %0.2f).CDF(0))", hashratePortion), line)

		data = plotter.XYs{}

		line, _ = plotter.NewLine(data2)
		line.Color = colornames.Blue200
		p.Add(line)
		p.Legend.Add(fmt.Sprintf("(1 - Poisson(1/13).CDF(0)) * %0.2f", hashratePortion), line)

		data2 = plotter.XYs{}
	}

	if err := p.Save(800, 600, "poisson_3_lambda.png"); err != nil {
		t.Fatal(err)
	}
}
