package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/montanaflynn/stats"
	d "github.com/whilei/empirical-ETH/data"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
)

func main() {
	p := plot.New()
	p.Title.Text = "Ethereum (ETH) Block Interval Distribution (blocks 13M..14M)"
	p.X.Label.Text = "Interval in Seconds (derived as timestamp offset)"
	p.Y.Label.Text = "Number of occurrences"

	data := map[int]int{}  // interval buckets
	dataP := plotter.XYs{} // typed buckets for plotting

	intervals, err := d.ReadDatasetIntervals()
	if err != nil {
		log.Fatalln(err)
	}

	for _, interval := range intervals {
		intervalInt := int(interval)

		// Add it to our bucket map.
		if v, ok := data[intervalInt]; !ok {
			data[intervalInt] = 1
		} else {
			data[intervalInt] = v + 1
		}
	}

	// Typed XY values from the data for the plotter lib.
	seconds := 0
	blocks := 0
	for k, v := range data {
		dataP = append(dataP, plotter.XY{X: float64(k), Y: float64(v)})

		seconds += k * v
		blocks += v
	}

	hist, _ := plotter.NewHistogram(dataP, len(data))
	p.Add(hist)

	outPath := filepath.Join("vis.png")
	if err := p.Save(800, 400, outPath); err != nil {
		log.Fatalln(err)
	}

	// Statistics

	mean := float64(seconds) / float64(blocks)
	fmt.Println("mean interval", mean)
	// => mean interval 13.482376

	min, _ := stats.Min(intervals)
	max, _ := stats.Max(intervals)
	med, _ := stats.Median(intervals)
	mmean, _ := stats.Mean(intervals)
	fmt.Println("min", min, "max", max, "med", med, "mean", mmean)
	// => min 1 max 208 med 9 mean 13.482376
}
