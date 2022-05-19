package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/montanaflynn/stats"
	"github.com/whilei/go-tabs-scraper/lib"
)

func main() {
	datadir := flag.String("datadir", "data", "Root data directory. Will be created if not existing.")
	flag.Parse()

	matches, err := filepath.Glob(filepath.Join(*datadir, "block_*"))
	if err != nil {
		log.Fatalln(err)
	}

	tabs := []*big.Float{}
	for mi, m := range matches {
		log.Printf("Reading match %d/%d %s\n", mi, len(matches), m)

		b, err := ioutil.ReadFile(m)
		if err != nil {
			log.Fatalln(err)
		}

		ap := &lib.AppBlock{}
		err = json.Unmarshal(b, ap)
		if err != nil {
			log.Fatalln(err)
		}

		tabs = append(tabs, mustGetPrettyTABWithMiner(ap))
	}

	tabsFloat64s := []float64{}
	for _, t := range tabs {
		f, _ := t.Float64()
		tabsFloat64s = append(tabsFloat64s, f)
	}

	median, _ := stats.Median(tabsFloat64s)
	mean, _ := stats.Mean(tabsFloat64s)
	max, _ := stats.Max(tabsFloat64s)
	min, _ := stats.Min(tabsFloat64s)
	p95, _ := stats.Percentile(tabsFloat64s, 95)
	p5, _ := stats.Percentile(tabsFloat64s, 5)

	fmt.Printf("samples %d\n", len(tabsFloat64s))
	fmt.Printf("median %0.3f\n", median)
	fmt.Printf("mean %0.3f\n", mean)
	fmt.Printf("max %0.3f\n", max)
	fmt.Printf("min %0.3f\n", min)
	fmt.Printf("p95 %0.3f\n", p95)
	fmt.Printf("p5 %0.3f\n", p5)
}

func mustGetPrettyTABWithMiner(ap *lib.AppBlock) *big.Float {
	if ap.TABWithMinerPretty != nil {
		return ap.TABWithMinerPretty
	}
	if ap.TABWithMiner != nil {
		return lib.PrettyBalance(ap.TABWithMiner)
	}
	out := new(big.Float)
	uniqueSenders := make(map[common.Address]bool)

	for _, tx := range ap.AppTxes {
		if _, ok := uniqueSenders[tx.From]; ok {
			continue
		} else {
			uniqueSenders[tx.From] = true
		}
		if tx.BalanceAtParentPretty != nil {
			out.Add(out, tx.BalanceAtParentPretty)
		} else {
			out.Add(out, lib.PrettyBalance(tx.BalanceAtParent))
		}
	}
	out.Add(out, lib.PrettyBalance(ap.MinerBalanceAtParent))
	return out
}
