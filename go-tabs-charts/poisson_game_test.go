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
	// var events = plotter.XYs{}

	// 0: first/genesis round
	// 14: an event happens at t=14, a new round begins
	for _, roundStartTime := range []float64{0, 14} {

		for i, minerShare := range []float64{redMinerShare, blueMinerShare} {
			for interval := float64(0); interval <= intervalSamples; interval++ {
				var poissonDist = distuv.Poisson{
					Lambda: interval * networkEventRate * minerShare,
					Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
				}
				y := 1 - poissonDist.CDF(0)
				data = append(data, plotter.XY{X: interval + roundStartTime, Y: y})

				// if interval == roundStartTime {
				// 	events = append(events, plotter.XY{X: interval, Y: y})
				// }
			}

			line, _ := plotter.NewLine(data)
			line.Color = minerColors[i]

			p.Add(line)
			p.Legend.Add(fmt.Sprintf("0+%d %0.3f", int(roundStartTime), minerShare), line)

			data = plotter.XYs{}

		}
	}
	//
	// eventsScatter, _ := plotter.NewScatter(events)
	// eventsScatter.Color = colornames.Black
	// eventsScatter.Shape = draw.CircleGlyph{}
	// eventsScatter.Radius = 3
	// p.Add(eventsScatter)

	fn := plotter.NewFunction(func(f float64) float64 {
		return
	})
	// fn.Samples = 10
	fn.Color = colornames.Black
	p.Add(fn)
	p.Legend.Add("name", fn)

	if err := p.Save(800, 600, "poisson_game_redblue.png"); err != nil {
		t.Fatal(err)
	}
}
