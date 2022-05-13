package main

import (
	"fmt"
	"image/color"
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

func TestChartPoissonGame(t *testing.T) {

	var networkEventRate float64 = float64(1) / float64(13)
	var redMinerShare, blueMinerShare float64 = float64(1) / 4, float64(1) / 8
	var minerColors = []color.Color{colornames.Red200, colornames.Blue200}
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

	var data = plotter.XYs{}

	for i, minerShare := range []float64{redMinerShare, blueMinerShare} {
		for interval := float64(0); interval <= intervalSamples; interval++ {
			var poissonDist = distuv.Poisson{
				Lambda: interval * networkEventRate * minerShare,
				Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
			}
			data = append(data, plotter.XY{X: interval, Y: 1 - poissonDist.CDF(0)})
		}

		line, _ := plotter.NewLine(data)
		line.Color = minerColors[i]

		p.Add(line)
		p.Legend.Add(fmt.Sprintf("%0.2f", minerShare), line)

		data = plotter.XYs{}
	}

	if err := p.Save(800, 600, "poisson_game_redblue.png"); err != nil {
		t.Fatal(err)
	}
}
