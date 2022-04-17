package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path/filepath"
	"strconv"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
)

func main() {
	p := plot.New()
	p.Title.Text = "Ethereum Classic (ETC) Block Interval Distribution (blocks 13M..14M)"
	p.X.Label.Text = "Interval in Seconds (derived as timestamp offset)"
	p.Y.Label.Text = "Number of occurrences"

	dataFilePath := filepath.Join("block-intervals.js.output.intervals.json")
	b, err := ioutil.ReadFile(dataFilePath)
	if err != nil {
		log.Fatalln(err)
	}

	data := map[string]int{}
	err = json.Unmarshal(b, &data)
	if err != nil {
		log.Fatalln(err)
	}

	dataP := plotter.XYs{}
	for k, v := range data {
		i, err := strconv.Atoi(k)
		if err != nil {
			log.Fatalln(err)
		}
		dataP = append(dataP, plotter.XY{X: float64(i), Y: float64(v)})
	}

	hist, _ := plotter.NewHistogram(dataP, len(data))
	p.Add(hist)

	outPath := filepath.Join("vis.png")
	if err := p.Save(800, 400, outPath); err != nil {
		log.Fatalln(err)
	}
}
