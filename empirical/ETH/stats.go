package main

import (
	"fmt"
	"log"

	"github.com/whilei/empirical-ETH/data"
)

func main() {
	intervals, err := data.ReadDatasetIntervals()
	if err != nil {
		log.Fatalln(err)
	}

	// How many intervals are in first bucket?
	bucketMin := float64(1)
	bucketMax := float64(9)

	hits := float64(0)
	for _, interval := range intervals {
		if interval <= bucketMax && interval >= bucketMin {
			hits++
		}
	}

	fmt.Printf("bucket1: %0.2f\n", hits/float64(len(intervals)))
}
