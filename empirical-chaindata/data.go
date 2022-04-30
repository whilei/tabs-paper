package main

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/csv"
	"log"
	"time"
)

//go:embed data/ETH/bq-results-20220417-161552-1650212199157.csv.gz
var ETHData []byte

//go:embed data/ETC/block-intervals.js.output.intervals.json
var ETCData []byte

func ReadDatasetIntervals(data []byte) ([]float64, error) {
	reader := bytes.NewReader(data)
	greader, err := gzip.NewReader(reader)
	if err != nil {
		log.Fatalln(err)
	}

	csvReader := csv.NewReader(greader)
	csvReader.FieldsPerRecord = 4
	/*
		number,hash,timestamp,difficulty
		13000000,0x736048fc56ee5570d18fce0fbad513f8a3cc1de2b18bfecfc8b3663e0bee1570,2021-08-10 21:53:39 UTC,8050151966801941
		13000001,0x26ff7321f0c5642b73c35dbd334c93a98585b2f7cc7122dcf89f693d4503f85c,2021-08-10 21:53:41 UTC,8054084852550629
		13000002,0x688484f241251654f382bdd6343f174e38eddd633336a15b2154af81a8c1ac10,2021-08-10 21:54:09 UTC,8046221682795459

	*/
	records, err := csvReader.ReadAll()

	datasetIntervalsFloat64s := []float64{}

	var lastRecordTime time.Time
	for i, record := range records {
		if i == 0 {
			continue // skip header row
		}
		// if i > 100 {
		// 	break // development/debug
		// }
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
		datasetIntervalsFloat64s = append(datasetIntervalsFloat64s, interval)

		// Set up the next block in mem.
		lastRecordTime = timestamp
	}

	return datasetIntervalsFloat64s, nil
}
