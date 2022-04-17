package main

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"time"

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

	dataFilePath := filepath.Join("bq-results-20220417-161552-1650212199157.csv")
	fi, err := os.OpenFile(dataFilePath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Fatalln(err)
	}
	csvReader := csv.NewReader(fi)
	csvReader.FieldsPerRecord = 4
	records, err := csvReader.ReadAll()

	var lastRecordTime time.Time
	for i, record := range records {
		if i == 0 {
			continue // skip header row
		}
		// if i > 100 {
		// 	break // development/debug
		// }
		/*
			number,hash,timestamp,difficulty
			13000000,0x736048fc56ee5570d18fce0fbad513f8a3cc1de2b18bfecfc8b3663e0bee1570,2021-08-10 21:53:39 UTC,8050151966801941
			13000001,0x26ff7321f0c5642b73c35dbd334c93a98585b2f7cc7122dcf89f693d4503f85c,2021-08-10 21:53:41 UTC,8054084852550629
			13000002,0x688484f241251654f382bdd6343f174e38eddd633336a15b2154af81a8c1ac10,2021-08-10 21:54:09 UTC,8046221682795459
		*/
		dateField := record[2]
		timestamp, err := time.Parse("2006-01-02 15:04:05 UTC", dateField)
		if err != nil {
			log.Fatalln(err)
		}

		// Set up the first calculation.
		if lastRecordTime.IsZero() {
			lastRecordTime = timestamp
			continue
		}

		// Calculate the time offset interval.
		interval := timestamp.Sub(lastRecordTime).Seconds()
		intervalInt := int(interval)

		// Add it to our bucket map.
		if v, ok := data[intervalInt]; !ok {
			data[intervalInt] = 1
		} else {
			data[intervalInt] = v + 1
		}

		// Set up the next block in mem.
		lastRecordTime = timestamp
	}

	fi.Close()

	// Typed XY values from the data for the plotter lib.
	for k, v := range data {
		dataP = append(dataP, plotter.XY{X: float64(k), Y: float64(v)})
	}

	hist, _ := plotter.NewHistogram(dataP, len(data))
	p.Add(hist)

	outPath := filepath.Join("vis.png")
	if err := p.Save(800, 400, outPath); err != nil {
		log.Fatalln(err)
	}
}
