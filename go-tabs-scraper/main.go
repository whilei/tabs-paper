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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	datadir := flag.String("datadir", "data", "Root data directory. Will be created if not existing.")
	spanStart := flag.Uint64("span-start", 14_000_000, "Start of block query span (default=14,000,000")
	spanEnd := flag.Uint64("span-end", 14_000_000+1000, "End of block query span (default=14,001,000)")
	spanRate := flag.Float64("span-rate", 1.0, "Rate of blocks query within span (default=1.0)")
	url := flag.String("url", "http://127.0.0.1:8545", "Ethclient endpoint URL")
	flag.Parse()

	if err := os.MkdirAll(*datadir, os.ModePerm); err != nil {
		log.Fatalln(err)
	} else {
		log.Println("OK: mkdir -p", *datadir)
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

	// This is the latest signer.
	// It is expected to be backwards-compatible for all signers.
	signer := types.NewLondonSigner(chainID)

	for blockN := *spanStart; blockN <= *spanEnd; blockN++ {

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
			Block:                bl,
			MinerBalanceAtParent: minerBalanceAtParent,
			AppTxes:              []AppTx{},
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

			// Bundle it up nice in our app data type.
			at := AppTx{
				Transaction:     tx,
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

type AppBlock struct {
	*types.Block
	MinerBalanceAtParent *big.Int
	AppTxes              []AppTx
}

type AppTx struct {
	*types.Transaction
	From            common.Address
	BalanceAtParent *big.Int
	Index           int
}

