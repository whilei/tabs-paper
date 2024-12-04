package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/mazznoer/colorgrad"
	exprand "golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestChartPoissonLambdas(t *testing.T) {
	// 1/14, 1/14/2, 1/14/4, 1/14/6, 1/14/8, 1/14/16
	var maxLambda float64 = float64(1) / float64(13)
	var lambdaSamples float64 = 32
	var intervalSamples float64 = 99

	p, _ := plot.New()
	p.Title.Text = "Poisson: Expected Rates of Block(s) Discovery over Time across Production Rate"

	p.X.Label.Text = "Time"
	p.X.Min = 0
	p.X.Max = intervalSamples
	p.X.Tick.Marker = DifficultyModTickerInterval{}

	p.Y.Label.Text = "Probability"
	p.Y.Min = 0
	p.Y.Max = 1

	grad := colorgrad.Viridis()
	colorMin, colorMax := grad.Domain()
	colorDomain := colorMax - colorMin

	var data = plotter.XYs{}

	for lambdaDivisor := float64(1); lambdaDivisor <= lambdaSamples; lambdaDivisor++ {
		for interval := float64(0); interval <= intervalSamples; interval++ {
			var poissonDist = distuv.Poisson{
				Lambda: interval * maxLambda / lambdaDivisor,
				Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
			}
			data = append(data, plotter.XY{X: interval, Y: 1 - poissonDist.CDF(0)})
		}

		line, _ := plotter.NewLine(data)
		line.Color = grad.At(lambdaDivisor * colorDomain / lambdaSamples)

		p.Add(line)
		p.Legend.Add(fmt.Sprintf("1/13/%d", int(lambdaDivisor)), line)

		data = plotter.XYs{}
	}

	if err := p.Save(800, 600, "poisson_lambda.png"); err != nil {
		t.Fatal(err)
	}
}
