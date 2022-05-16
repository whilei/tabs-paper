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
	"github.com/ethereum/go-ethereum/params"
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
		ap := AppBlock{
			Header:               bl.Header(),
			TxesN:                bl.Transactions().Len(),
			MinerBalanceAtParent: minerBalanceAtParent,
			AppTxes:              []AppTx{},

			TABWithMiner:       new(big.Int).Set(minerBalanceAtParent),
			TABWithMinerPretty: prettyBalance(minerBalanceAtParent),

			TABWithoutMiner:       new(big.Int),
			TABWithoutMinerPretty: new(big.Float),
		}

		for txi, tx := range bl.Transactions() {

			// Extract the 'from' address using the presumed signer.
			msg, err := tx.AsMessage(signer, bl.BaseFee())
			if err != nil {
				log.Fatalln(err)
			}
			from := msg.From()

			// Get her balance.
			bal, err := client.BalanceAt(bkgrnd, from, parentN)
			if err != nil {
				log.Fatalln(err)
			}

			ap.TABWithoutMiner.Add(ap.TABWithoutMiner, bal)
			ap.TABWithoutMinerPretty.Add(ap.TABWithoutMinerPretty, prettyBalance(bal))

			ap.TABWithMiner.Add(ap.TABWithMiner, bal)
			ap.TABWithMinerPretty.Add(ap.TABWithMinerPretty, prettyBalance(bal))

			// Bundle it up nice in our app data type.
			at := AppTx{
				CTransaction:    tx,
				Index:           txi,
				From:            from,
				BalanceAtParent: bal,
			}
			ap.AppTxes = append(ap.AppTxes, at)
		}

		// Extraction logic done for this block.

		// Persist the data.

		j, err := json.MarshalIndent(ap, "", "    ")
		if err != nil {
			log.Fatalln(err)
		}
		blockFilePath := filepath.Join(*datadir, fmt.Sprintf("block_%v", blockN))
		if err := ioutil.WriteFile(blockFilePath, j, os.ModePerm); err != nil {
			log.Fatalln(err)
		}
	}
}

var etherBig = big.NewFloat(params.Ether)

func prettyBalance(bal *big.Int) *big.Float {
	return new(big.Float).Quo(new(big.Float).SetInt(bal), etherBig)
}

type AppBlock struct {
	Header                *types.Header
	TxesN                 int
	MinerBalanceAtParent  *big.Int
	AppTxes               []AppTx
	TABWithoutMiner       *big.Int
	TABWithoutMinerPretty *big.Float
	TABWithMiner          *big.Int
	TABWithMinerPretty    *big.Float
}

type AppTx struct {
	CTransaction          *types.Transaction
	From                  common.Address
	BalanceAtParent       *big.Int
	BalanceAtParentPretty *big.Float
	Index                 int
}
