package main

import (
	"fmt"
	"log"
)

func BucketShare(data []byte, start, stop float64) {
	intervals, err := ReadDatasetIntervals(data)
	if err != nil {
		log.Fatalln(err)
	}

	// How many intervals are in first bucket?
	bucketMin := start
	bucketMax := stop

	hits := float64(0)
	for _, interval := range intervals {
		if interval <= bucketMax && interval >= bucketMin {
			hits++
		}
	}

	fmt.Printf("bucket: %0.2f\n", hits/float64(len(intervals)))
}
