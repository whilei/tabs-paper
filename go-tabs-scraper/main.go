/*
	By default, the query space will be derived backwards from the latest block number reported by the client (via the spanRange).
	By default, the query will be sampled sequentially for the duration of this block range.

	The spanRange variable allows the user to randomly query blocks at some rate.
	This builds an opportunity for non-sequential sampling.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/whilei/go-tabs-scraper/lib"
)

func main() {
	datadir := flag.String("datadir", "data", "Root data directory. Will be created if not existing.")
	spanStart := flag.Uint64("span-start", math.MaxInt64, "Start of block query span (default=latest-1")
	spanRange := flag.Uint64("span-range", 1, "Duration in blocks of block query span (default=1)")
	spanRate := flag.Float64("span-rate", 1.0, "Rate of blocks query within span (default=1.0)")
	url := flag.String("url", "http://127.0.0.1:8545", "Ethclient endpoint URL")
	flag.Parse()

	if err := os.MkdirAll(*datadir, os.ModePerm); err != nil {
		log.Fatalln(err)
	} else {
		log.Println("OK: mkdir -p", *datadir)
	}

	if *spanRate > 1 || *spanRate < 0 {
		log.Fatalln("impossible span rate:", *spanRate, "maximum = 1, minimum = 0")
	}

	client, err := ethclient.Dial(*url)
	if err != nil {
		log.Fatalln(err)
	}
	bkgrnd := context.Background()

	chainID, err := client.ChainID(bkgrnd)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("OK: chainid=%d\n", chainID)
	if err := ioutil.WriteFile(filepath.Join(*datadir, "chainid"), []byte(fmt.Sprintf("%d", chainID)), os.ModePerm); err != nil {
		log.Fatalln(err)
	}

	if *spanStart == math.MaxInt64 {
		latest, err := client.BlockNumber(bkgrnd)
		if err != nil {
			log.Fatalln(err)
		}
		*spanStart = latest - *spanRange
	}
	log.Printf("* start=%d range=%d end=%d rate=%0.3f\n", *spanStart, *spanRange, *spanStart+*spanRange, *spanRate)

	// This is the latest signer.
	// It is expected to be backwards-compatible for all signers.
	signer := types.NewLondonSigner(chainID)

	for blockN := *spanStart; blockN <= *spanStart+*spanRange; blockN++ {
		blockFilePath := filepath.Join(*datadir, fmt.Sprintf("block_%v", blockN))
		if fi, err := os.Stat(blockFilePath); err == nil || os.IsExist(err) {
			log.Println("Data exists, skipping", fi.Name())
			continue
		}

		if rand.Float64() > *spanRate {
			continue
		}
		log.Printf("Querying block.n=%d\n", blockN)

		// Get the block
		bl, err := client.BlockByNumber(bkgrnd, big.NewInt(int64(blockN)))
		if err != nil {
			log.Fatalln(err)
		}

		// Get the miner's balance at the parent block
		parentN := big.NewInt(int64(blockN) - 1)
		minerBalanceAtParent, err := client.BalanceAt(bkgrnd, bl.Coinbase(), parentN)
		if err != nil {
			log.Fatalln(err)
		}

		// Bundle it up in our app data type.
		ap := lib.AppBlock{
			Header:               bl.Header(),
			TxesN:                bl.Transactions().Len(),
			MinerBalanceAtParent: minerBalanceAtParent,
			AppTxes:              []lib.AppTx{},

			TABWithMiner:       new(big.Int).Set(minerBalanceAtParent),
			TABWithMinerPretty: lib.PrettyBalance(minerBalanceAtParent),

			TABWithoutMiner:       new(big.Int),
			TABWithoutMinerPretty: new(big.Float),
		}

		uniqueSenders := make(map[common.Address]bool)

		for txi, tx := range bl.Transactions() {

			// Extract the 'from' address using the presumed signer.
			msg, err := tx.AsMessage(signer, bl.BaseFee())
			if err != nil {
				log.Fatalln(err)
			}
			from := msg.From()
			if _, ok := uniqueSenders[from]; ok {
				continue
			} else {
				uniqueSenders[from] = true
			}

			// Get her balance.
			bal, err := client.BalanceAt(bkgrnd, from, parentN)
			if err != nil {
				log.Fatalln(err)
			}

			prettyBal := lib.PrettyBalance(bal)
			ap.TABWithoutMiner.Add(ap.TABWithoutMiner, bal)
			ap.TABWithoutMinerPretty.Add(ap.TABWithoutMinerPretty, prettyBal)

			ap.TABWithMiner.Add(ap.TABWithMiner, bal)
			ap.TABWithMinerPretty.Add(ap.TABWithMinerPretty, prettyBal)

			// Bundle it up nice in our app data type.
			at := lib.AppTx{
				CTransaction:          tx,
				Index:                 txi,
				From:                  from,
				BalanceAtParent:       bal,
				BalanceAtParentPretty: prettyBal,
			}
			ap.AppTxes = append(ap.AppTxes, at)
		}

		// Extraction logic done for this block.

		// Persist the data.

		j, err := json.MarshalIndent(ap, "", "    ")
		if err != nil {
			log.Fatalln(err)
		}

		if err := ioutil.WriteFile(blockFilePath, j, os.ModePerm); err != nil {
			log.Fatalln(err)
		}
	}
}
