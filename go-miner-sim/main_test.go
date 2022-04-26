package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// TestBlockTree_AppendBlock is a unit test.
func TestBlockTree_AppendBlock(t *testing.T) {
	bt := NewBlockTree()
	bt.AppendBlockByNumber(genesisBlock)
	if len(bt[0]) == 0 {
		t.Fatal("missing genesis at index=0")
	}
	bt.AppendBlockByNumber(&Block{
		i: 1,
	})
	if len(bt[1]) == 0 {
		t.Fatal("missing block i=1 at index=1")
	}
}

func TestProcessBlock(t *testing.T) {
	m := &Miner{
		// ConsensusAlgorithm: TDTABS,
		// ConsensusAlgorithm: TD,
		Index:         0,
		Address:       "exampleMiner", // avoid collisions
		HashesPerTick: 42,
		Balance:       42000000,
		// BalanceCap:               minerStartingBalance,
		Blocks:                   NewBlockTree(),
		head:                     nil,
		receivedBlocks:           BlockTree{},
		neighbors:                []*Miner{},
		reorgs:                   make(map[int64]struct{ add, drop int }),
		decisionConditionTallies: make(map[string]int),
		cord:                     make(chan minerEvent),
		Delay: func() int64 {
			return int64(delaySecondsDefault * float64(ticksPerSecond))
			// return int64(hr * 3 * rand.Float64() * float64(ticksPerSecond))
		},
		Latency: func() int64 {
			return int64(latencySecondsDefault * float64(ticksPerSecond))
			// return int64(4 * float64(ticksPerSecond))
			// return int64((4 * rand.Float64()) * float64(ticksPerSecond))
		},
	}

	// goroutine reads miner events chan (cord)
	go func() {
		for range m.cord {
		}
	}()

	m.processBlock(genesisBlock) // sets head to genesis

	ph := genesisBlock.h
	for i := int64(1); i < 10; i++ {
		b := &Block{i: i, canonical: true, ph: ph, h: fmt.Sprintf("%08x", rand.Int63())}
		ph = b.h
		m.Blocks.AppendBlockByNumber(b)
		m.setHead(b)
	}

	b := &Block{i: 8, canonical: true, ph: m.Blocks.GetBlockByNumber(7).h, h: fmt.Sprintf("%08x", rand.Int63())}
	m.Blocks.AppendBlockByNumber(b)
	m.setHead(b)

	b = &Block{i: 9, canonical: true, ph: b.h, h: fmt.Sprintf("%08x", rand.Int63())}
	m.Blocks.AppendBlockByNumber(b)
	m.setHead(b)

	b = &Block{i: 10, canonical: true, ph: b.h, h: fmt.Sprintf("%08x", rand.Int63())}
	m.Blocks.AppendBlockByNumber(b)
	m.setHead(b)

	t.Log(m.Blocks.String())
}
