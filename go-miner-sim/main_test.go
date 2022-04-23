package main_test

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fogleman/gg"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/montanaflynn/stats"
	"golang.org/x/image/colornames"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"

	"github.com/mazznoer/colorgrad"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Miner struct {
	Index   int64
	Address string
	Blocks  BlockTree

	HashesPerTick int64 // per tick
	Balance       int64 // Wei
	BalanceCap    int64 // Max Wei this miner will hold. Use 0 for no limit hold 'em.
	// CostPerBlock  int64 // cost to miner, expended after each block win (via tx on text block)

	Latency func() int64
	Delay   func() int64

	ConsensusAlgorithm             ConsensusAlgorithm
	ConsensusArbitrations          int
	ConsensusObjectiveArbitrations int

	reorgs                   map[int64]struct{ add, drop int }
	decisionConditionTallies map[string]int

	head *Block

	neighbors      []*Miner
	receivedBlocks map[int64]Blocks

	cord chan minerEvent

	tick int64
}

func getBlockDifficulty(parent *Block, uncles bool, interval int64) int64 {
	x := interval / (9 * ticksPerSecond) // 9 SECONDS
	y := 1 - x
	if uncles {
		y = 2 - x
	}
	if y < -99 {
		y = -99
	}
	return int64(float64(parent.d) + (float64(y) / 2048 * float64(parent.d)))
}

func getTABS(parent *Block, localTAB int64) int64 {
	scalarNumerator := int64(0)
	if localTAB > parent.tabs {
		scalarNumerator = 1
	} else if localTAB < parent.tabs {
		scalarNumerator = -1
	}

	numerator := tabsAdjustmentDenominator + scalarNumerator // [127|128|129]/128, [4095|4096|4097]/4096

	return int64(float64(parent.tabs) * float64(numerator) / float64(tabsAdjustmentDenominator))
}

func (m *Miner) mineTick() {
	parent := m.head

	// - HashesPerTick / parent.difficulty gives relative network hashrate share
	// - * m.Lambda gives relative trial share per tick
	tickR := float64(m.HashesPerTick) / float64(parent.d) * networkLamda
	tickR = tickR / 2

	// Do we solve it?
	needle := rand.Float64()
	trial := rand.Float64()

	if math.Abs(trial-needle) <= tickR ||
		math.Abs(trial-needle) >= 1-tickR {

		// Naively, the block tick is the miner's real tick.
		s := m.tick

		// But if the tickInterval allows multiple ticks / second,
		// we need to enforce that the timestamp is a unit-second value.
		s = s / ticksPerSecond // floor
		s = s * ticksPerSecond // back to interval units

		// In order for the block to be valid, the tick must be greater
		// than that of its parent.
		if s == parent.s {
			s = parent.s + 1
		}

		// A naive model of uncle references: bool=yes if any orphan blocks exist in our miner's record of blocks
		uncles := len(m.Blocks[parent.i]) > 1

		blockDifficulty := getBlockDifficulty(parent /* interval: */, uncles, s-parent.s)
		tabs := getTABS(parent, m.Balance)
		tdtabs := tabs * blockDifficulty
		b := &Block{
			i:       parent.i + 1,
			s:       s, // miners are always honest about their timestamps
			si:      s - parent.s,
			d:       blockDifficulty,
			td:      parent.td + blockDifficulty,
			tabs:    tabs,
			ttdtabs: parent.ttdtabs + tdtabs,
			miner:   m.Address,
			ph:      parent.h,
			h:       fmt.Sprintf("%08x", rand.Int63()),
		}
		m.processBlock(b)
		m.broadcastBlock(b)
	}
}

func (m *Miner) broadcastBlock(b *Block) {
	b.delay = Delay{
		subjective: m.Delay(),
		material:   m.Latency(),
	}
	for _, n := range m.neighbors {
		n.receiveBlock(b)
	}
}

func (m *Miner) receiveBlock(b *Block) {
	if d := b.delay.Total(); d > 0 {
		if len(m.receivedBlocks[b.s+d]) > 0 {
			m.receivedBlocks[b.s+d] = append(m.receivedBlocks[b.s+d], b)
		} else {
			m.receivedBlocks[b.s+d] = Blocks{b}
		}
		return
	}
	m.processBlock(b)
}

func (m *Miner) doTick(s int64) {
	m.tick = s

	// Get tick-expired received blocks and process them.
	for k, v := range m.receivedBlocks {
		if m.tick >= k && /* future block inhibition */ m.tick+(15*ticksPerSecond) > k {
			for _, b := range v {
				m.processBlock(b)
			}
			delete(m.receivedBlocks, k)
		}
	}

	// Mine.
	m.mineTick()
}

func (m *Miner) processBlock(b *Block) {
	dupe := m.Blocks.AppendBlockByNumber(b)
	if !dupe {
		defer m.broadcastBlock(b)
	}

	// Special case: init genesis block.
	if m.head == nil {
		m.head = b
		m.head.canonical = true
		return
	}

	canon := m.arbitrateBlocks(m.head, b)
	m.setHead(canon)
}

func (m *Miner) incrementMinerBalance(i int64) {
	m.Balance += i
	if m.BalanceCap != 0 && m.Balance > m.BalanceCap {
		m.Balance = m.BalanceCap
	}
}

func (m *Miner) setHead(head *Block) {
	// Should never happen, but handle the case.
	if m.head.h == head.h {
		return
	}

	doReorg := m.head.h != head.ph
	if doReorg {
		// Reorg!
		add, drop := 1, 0

		ph := head.ph
	outer:
		for i := head.i - 1; i > 0; i-- {
			for _, b := range m.Blocks[i] {
				if b.canonical && b.h == ph {
					break outer

				} else if b.canonical {
					if b.miner == m.Address {
						m.incrementMinerBalance(-blockReward)
					}
					drop++
					b.canonical = false
				} else if !b.canonical && b.h == ph {
					if b.miner == m.Address {
						m.incrementMinerBalance(blockReward)
					}
					add++
					b.canonical = true
					ph = b.ph
				}
			}
		}
		for _, b := range m.Blocks[head.i] {
			if b.h != head.h {
				if b.canonical {
					if b.miner == m.Address {
						m.incrementMinerBalance(-blockReward)
					}
					drop++
					b.canonical = false
				}
			}
		}
		for i := head.i + 1; ; i++ {
			if len(m.Blocks[i]) == 0 {
				break
			}
			for _, b := range m.Blocks[i] {
				if b.canonical {
					if b.miner == m.Address {
						m.incrementMinerBalance(-blockReward)
					}
					drop++
					b.canonical = false
				}
			}
		}

		m.reorgs[head.i] = struct{ add, drop int }{add, drop}

		// fmt.Println("Reorg!", m.Address, head.i, "add", add, "drop", drop)
	}

	m.head = head
	m.head.canonical = true

	// Block reward. Block-transaction fees are held presumed constant.
	if m.Address == head.miner {
		m.incrementMinerBalance(blockReward)
	}

	headI := head.i

	m.cord <- minerEvent{
		minerI: int(m.Index),
		i:      headI,
		blocks: m.Blocks[headI],
	}
}

func (m *Miner) reorgMagnitudes() (magnitudes []float64) {
	for k := range m.Blocks {
		// This takes reorg magnitudes for ALL blocks,
		// not just the block numbers at which reorgs happened.
		// TODO
		if v, ok := m.reorgs[k]; ok {
			magnitudes = append(magnitudes, float64(v.add+v.drop))
		}
	}
	return magnitudes
}

// arbitrateBlocks selects one canonical block from any two blocks.
func (m *Miner) arbitrateBlocks(a, b *Block) *Block {
	m.ConsensusArbitrations++          // its what we do here
	m.ConsensusObjectiveArbitrations++ // an assumption that will be undone (--) if it does not hold

	decisionCondition := "pow_score_high"
	defer func() {
		m.decisionConditionTallies[decisionCondition]++
	}()

	if m.ConsensusAlgorithm == TD {
		// TD arbitration
		if a.td > b.td {
			return a
		} else if b.td > a.td {
			return b
		}
	} else if m.ConsensusAlgorithm == TDTABS {
		if (a.ttdtabs) > (b.ttdtabs) {
			return a
		} else if (b.ttdtabs) > (a.ttdtabs) {
			return b
		}
	}

	// Number arbitration
	decisionCondition = "height_low"
	if a.i < b.i {
		return a
	} else if b.i < a.i {
		return b
	}

	// If we've reached this point, the arbitration was not
	// objective.
	m.ConsensusObjectiveArbitrations--

	// Self-interest arbitration
	decisionCondition = "miner_selfish"
	if a.miner == m.Address && b.miner != m.Address {
		return a
	} else if b.miner == m.Address && a.miner != m.Address {
		return b
	}

	// Coin toss
	decisionCondition = "random"
	if rand.Float64() < 0.5 {
		return a
	}
	return b
}

type ConsensusAlgorithm int

const (
	None ConsensusAlgorithm = iota
	TD
	TDTABS
	TimeAsc
	TimeDesc // FreshnessPreferred
)

func (c ConsensusAlgorithm) String() string {
	switch c {
	case TD:
		return "TD"
	case TDTABS:
		return "TDTABS"
	case TimeAsc:
		return "TimeAsc"
	case TimeDesc:
		return "TimeDesc"
	}
	panic("impossible")
}

type Block struct {
	i         int64  // H_i: number
	s         int64  // H_s: timestamp
	si        int64  // interval
	d         int64  // H_d: difficulty
	td        int64  // H_td: total difficulty
	tabs      int64  // H_k: TAB synthesis
	ttdtabs   int64  // H_k: TTABSConsensusScore, aka Total TD*TABS
	miner     string // H_c: coinbase/etherbase/author/beneficiary
	h         string // H_h: hash
	ph        string // H_p: parent hash
	canonical bool

	delay Delay
}

type Delay struct {
	subjective int64
	material   int64
}

func (d Delay) Total() int64 {
	return d.subjective + d.material
}

type Blocks []*Block
type BlockTree map[int64]Blocks

func NewBlockTree() BlockTree {
	return BlockTree(make(map[int64]Blocks))
}

func (bt BlockTree) AppendBlockByNumber(b *Block) (dupe bool) {
	if _, ok := bt[b.i]; !ok {
		// Is new block for number i
		bt[b.i] = Blocks{b}
		return false
	} else {
		// Is competitor block for number i

		for _, bb := range bt[b.i] {
			if b.h == bb.h {
				dupe = true
			}
		}
		if !dupe {
			bt[b.i] = append(bt[b.i], b)
		}
	}
	return dupe
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

// Ks returns a slice of K tallies (number of available blocks) for each block number.
// It weirdly returns a float64 because it will be used with stats packages
// that like []float64.
func (bt BlockTree) Ks() (ks []float64) {
	for _, v := range bt {
		if len(v) == 0 {
			panic("how?")
		}
		ks = append(ks, float64(len(v)))
	}
	return ks
}

// Intervals returns ALL block intervals for a tree (whether canonical or not).
// Again, []float64 is used because its convenient in context.
func (bt BlockTree) CanonicalIntervals() (intervals []float64) {
	for _, v := range bt {
		for _, b := range v {
			if b.canonical {
				intervals = append(intervals, float64(b.si))
			}
		}
	}
	return intervals
}

func (bt BlockTree) CanonicalDifficulties() (difficulties []float64) {
	for _, v := range bt {
		for _, b := range v {
			if !b.canonical {
				continue
			}
			difficulties = append(difficulties, float64(b.d))
		}
	}
	return difficulties
}

func (bt BlockTree) GetBlockByNumber(i int64) *Block {
	for _, bl := range bt[i] {
		if bl.canonical {
			return bl
		}
	}
	return nil
}

func (bt BlockTree) GetSideBlocksByNumber(i int64) (sideBlocks Blocks) {
	for _, bl := range bt[i] {
		if !bl.canonical {
			sideBlocks = append(sideBlocks, bl)
		}
	}
	return sideBlocks
}

func (bt BlockTree) GetBlockByHash(h string) *Block {
	for i := int64(len(bt) - 1); i >= 0; i-- {
		for _, b := range bt[i] {
			if b.h == h {
				return b
			}
		}
	}
	return nil
}

// Globals
var ticksPerSecond int64 = 10
var tickSamples = ticksPerSecond * int64((time.Hour * 24).Seconds())
var networkLamda = (float64(1) / float64(13)) / float64(ticksPerSecond)
var countMiners = int64(12)
var minerNeighborRate float64 = 0.5 // 0.7
var blockReward int64 = 3

var latencySecondsDefault float64 = 2.5
var delaySecondsDefault float64 = 0

const tabsAdjustmentDenominator = int64(128)
const genesisBlockTABS int64 = 10_000 // tabs starting value
const genesisDifficulty = 10_000_000_000

var genesisBlock = &Block{
	i:         0,
	s:         0,
	d:         genesisDifficulty,
	td:        genesisDifficulty,
	tabs:      genesisBlockTABS,
	ttdtabs:   genesisBlockTABS * genesisDifficulty,
	miner:     "X",
	delay:     Delay{},
	h:         fmt.Sprintf("%08x", rand.Int63()),
	canonical: true,
}

type minerResults struct {
	ConsensusAlgorithm ConsensusAlgorithm
	HashrateRel        float64
	HeadI              int64
	HeadTABS           int64

	KMean                      float64
	IntervalsMeanSeconds       float64
	DifficultiesRelGenesisMean float64

	Balance                 int64
	DecisiveArbitrationRate float64
	ReorgMagnitudesMean     float64
}

func TestPlotting(t *testing.T) {
	cases := []struct {
		name          string
		minerMutation func(m *Miner)
	}{
		{
			name: "td",
			minerMutation: func(m *Miner) {
				m.ConsensusAlgorithm = TD
			},
		},
		{
			name: "tdtabs",
			minerMutation: func(m *Miner) {
				m.ConsensusAlgorithm = TDTABS
			},
		},
	}

	for _, c := range cases {
		c := c
		runTestPlotting(t, c.name, c.minerMutation)
	}

	// runTestPlotting(t, "td", func(m *Miner) {
	// 	m.ConsensusAlgorithm = TD
	// })
}

type Miners []*Miner

func (ms Miners) headMax() (max int64) {
	for _, m := range ms {
		if m.head.i > max {
			max = m.head.i
		}
	}
	return max
}

type minerEvent struct {
	minerI int
	i      int64
	blocks Blocks
}

func runTestPlotting(t *testing.T, name string, mut func(m *Miner)) {

	t.Log("Running", name)

	outDir := filepath.Join("out", name)
	os.MkdirAll(outDir, os.ModePerm)
	os.RemoveAll(filepath.Join(outDir, "anim"))
	os.MkdirAll(filepath.Join(outDir, "anim"), os.ModePerm)

	miners := []*Miner{}
	minerEvents := make(chan minerEvent)

	hashrates := generateMinerHashrates(HashrateDistLongtail, int(countMiners))
	deriveMinerRelativeDifficultyHashes := func(genesisD int64, r float64) int64 {
		return int64(float64(genesisD) * r)
	}

	// We use relative hashrate as a proxy for balance;
	// more mining capital :: more currency capital.
	deriveMinerStartingBalance := func(genesisTABS int64, r float64) int64 {
		supply := genesisTABS * countMiners
		return int64((float64(supply) * r))
	}

	lastColor := colorful.Color{}
	grad := colorgrad.Viridis()

	for i := int64(0); i < countMiners; i++ {

		// set up their starting view of the chain
		bt := NewBlockTree()
		bt.AppendBlockByNumber(genesisBlock)

		// set up the miner

		minerStartingBalance := deriveMinerStartingBalance(genesisBlock.tabs, hashrates[i])
		hashes := deriveMinerRelativeDifficultyHashes(genesisBlock.d, hashrates[i])

		clr := grad.At(1 - (hashrates[i] * (1 / hashrates[0])))
		if clr == lastColor {
			// Make sure colors (names) are unique.
			clr.R++
		}
		lastColor = clr
		minerName := clr.Hex()[1:]

		// format := "#%02x%02x%02x"
		// minerName := fmt.Sprintf("%02x%02x%02x", clr.R, clr.G, clr.B)

		m := &Miner{
			// ConsensusAlgorithm: TDTABS,
			// ConsensusAlgorithm: TD,
			Index:                    i,
			Address:                  minerName, // avoid collisions
			HashesPerTick:            hashes,
			Balance:                  minerStartingBalance,
			BalanceCap:               minerStartingBalance,
			Blocks:                   bt,
			head:                     nil,
			receivedBlocks:           BlockTree{},
			neighbors:                []*Miner{},
			reorgs:                   make(map[int64]struct{ add, drop int }),
			decisionConditionTallies: make(map[string]int),
			cord:                     minerEvents,
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

		mut(m)

		m.processBlock(genesisBlock) // sets head to genesis
		miners = append(miners, m)
	}

	c := gg.NewContext(800, 1200)
	marginX, marginY := c.Width()/100, c.Width()/100

	c.Push()
	c.SetColor(colornames.White)
	c.DrawRectangle(0, 0, float64(c.Width()), float64(c.Height()))
	c.Fill()
	c.Stroke()
	c.Pop()

	c.SavePNG(filepath.Join(outDir, "anim", "out.png"))

	videoBlackRedForks := true

	go func() {
		c.Push() // unresolved state push

		for event := range minerEvents {
			// t.Log("minerEvent", event)

			// t.Log("here", 1)
			// t.Logf("event: %v", event)

			// ctx.DrawCircle(rand.Float64()*float64(c.Width), rand.Float64()*float64(c.Height), 10)

			xW := (c.Width() - (2 * marginX)) / int(countMiners)
			x := event.minerI*xW + marginX

			blockRowsN := 150
			yH := (c.Height() - (2 * marginY)) / blockRowsN
			var y int64
			// if event.i > blockRowsN{
			// 	y = 0
			// } else {
			y = int64(c.Height()) - (event.i%int64(blockRowsN))*int64(yH) + int64(marginY)
			// }

			// Clear the row above on bottom-up overlap/overdraw.
			c.Push()
			c.SetColor(colornames.White)
			c.DrawRectangle(0, float64(y-int64(yH*5)), float64(c.Width()), float64(yH*5))
			c.Fill()
			c.Stroke()
			c.Pop()

			// if event.i > 200 {
			// 	// c.Push()
			// 	// c.Translate(0, float64(yH))
			// 	// c.Stroke()
			// 	// c.Pop()
			// }

			nblocks := len(event.blocks)

			// // Outline competitive blocks for visibility..
			// if nblocks > 1 {
			// 	c.Push()
			// 	c.SetColor(colornames.Black)
			// 	c.SetLineWidth(1)
			// 	c.DrawRectangle(float64(x-2), float64(y-2), float64(xW+4), float64(yH+4))
			// 	// c.Fill()
			// 	c.Stroke()
			// 	c.Pop()
			// }

			// Or, more better, when you're interested in seeing forks,
			// just don't print the uncontested blocks.
			// if nblocks <= 1 {
			// 	continue
			// }

			for ib, b := range event.blocks {
				// t.Logf("push")
				c.Push()
				if videoBlackRedForks {
					// Black blocks = uncontested
					// Red   blocks = network forks
					clr := colornames.Black
					if nblocks > 1 {
						clr = colornames.Red
					}
					c.SetColor(clr)
				} else {
					// Get the block color from the block's authoring miner.
					clr, err := ParseHexColor("#" + b.miner)
					if err != nil {
						t.Log("bad color", err.Error())
						panic("test")
					}
					c.SetColor(clr)
				}

				realX := float64(x)
				realX += float64(ib) * float64(xW/nblocks)

				rectMargin := float64(0)

				rectX, rectY := realX+rectMargin, float64(y)+rectMargin
				rectW, rectH := float64(xW/nblocks)-(2*rectMargin), float64(yH)-(2*rectMargin)

				// t.Log("here.ib", ib, b == nil, b.miner, clr)
				// t.Logf("x=%d y=%d width=%v height=%v b=%d/%d", x, y, xW, yH, ib, nblocks)
				c.DrawRectangle(rectX, rectY, rectW, rectH)
				c.Fill()
				c.Stroke()
				c.Pop()
				// t.Logf("pop")
			}

			// t.Log("here", 2)

		}
	}()

	for i, m := range miners {
		for j, mm := range miners {
			if i == j {
				continue
			}
			if rand.Float64() < minerNeighborRate {
				m.neighbors = append(m.neighbors, mm)
			}
		}
	}

	lastHighBlock := int64(0)
	for s := int64(1); s <= tickSamples; s++ {
		for _, m := range miners {
			m.doTick(s)
		}
		if s%ticksPerSecond == 0 {
			// time.Sleep(time.Millisecond * 100)
		}
		nextHighBlock := Miners(miners).headMax()
		if nextHighBlock > lastHighBlock {
			// if s%ticksPerSecond == 0 {

			if err := c.SavePNG(filepath.Join(outDir, "anim", fmt.Sprintf("%04d_f.png", nextHighBlock))); err != nil {
				t.Fatal("save png errored", err)
			}

			lastHighBlock = nextHighBlock
			// 	// Human-readable intervals.
			//
			// 	line := ""
			//
			// 	for i, m := range miners {
			// 		fmt.Sprintf(`%s`,
			// 			strings.Repeat("\", i))
			// 	}
		}

		// TODO: measure network graphs? eg. bifurcation tally?
	}

	t.Log("RESULTS", name)

	for i, m := range miners {
		kMean, _ := stats.Mean(m.Blocks.Ks())
		kMed, _ := stats.Median(m.Blocks.Ks())
		kMode, _ := stats.Mode(m.Blocks.Ks())

		intervalsMean, _ := stats.Mean(m.Blocks.CanonicalIntervals())
		intervalsMean = intervalsMean / float64(ticksPerSecond)
		difficultiesMean, _ := stats.Mean(m.Blocks.CanonicalDifficulties())

		reorgMagsMean, _ := stats.Mean(m.reorgMagnitudes())

		t.Logf(`a=%s c=%s hr=%0.2f h/t=%d head.i=%d head.tabs=%d k_mean=%0.3f k_med=%0.3f k_mode=%v intervals_mean=%0.3fs d_mean.rel=%0.3f balance=%d objective_decs=%0.3f reorgs.mag_mean=%0.3f`,
			m.Address, m.ConsensusAlgorithm, hashrates[i], m.HashesPerTick,
			m.head.i, m.head.tabs,
			kMean, kMed, kMode,
			intervalsMean, difficultiesMean/float64(genesisBlock.d),
			m.Balance,
			float64(m.ConsensusObjectiveArbitrations)/float64(m.ConsensusArbitrations),
			reorgMagsMean)
	}

	t.Log("Making plots...")

	plotIntervals := func() {
		filename := filepath.Join(outDir, "sample_intervals.png")
		p := plot.New()

		buckets := map[int]int{}
		for _, blocks := range miners[0].Blocks {
			for _, b := range blocks {
				buckets[int(b.si/ticksPerSecond)]++
			}
		}
		data := plotter.XYs{}
		for k, v := range buckets {
			data = append(data, plotter.XY{X: float64(k), Y: float64(v)})
		}
		hist, err := plotter.NewHistogram(data, len(buckets))
		if err != nil {
			panic(err)
		}
		p.Add(hist)
		p.Save(800, 300, filename)
	}
	plotIntervals()

	plotDifficulty := func() {
		filename := filepath.Join(outDir, "block_difficulties.png")
		p := plot.New()

		data := plotter.XYs{}
		for k, v := range miners[0].Blocks {
			data = append(data, plotter.XY{X: float64(k), Y: float64(v[0].d)})
		}
		scatter, err := plotter.NewScatter(data)
		if err != nil {
			panic(err)
		}
		scatter.Radius = 1
		scatter.Shape = draw.CircleGlyph{}
		p.Add(scatter)
		p.Y.Min = float64(genesisBlock.d) / 2 // low enough for sense of scale of variance
		p.Save(800, 300, filename)
	}
	plotDifficulty()

	plotTABS := func() {
		filename := filepath.Join(outDir, "block_tabs.png")
		p := plot.New()

		data := plotter.XYs{}
		for k, v := range miners[0].Blocks {
			data = append(data, plotter.XY{X: float64(k), Y: float64(v[0].tabs)})
		}
		scatter, err := plotter.NewScatter(data)
		if err != nil {
			panic(err)
		}
		scatter.Radius = 1
		scatter.Shape = draw.CircleGlyph{}
		p.Add(scatter)
		p.Y.Min = float64(genesisBlock.tabs) / 2 // low enough for sense of scale of variance
		p.Save(800, 300, filename)
	}
	plotTABS()

	plotMinerTDs := func() {
		filename := filepath.Join(outDir, "miner_tds.png")
		p := plot.New()

		data := plotter.XYs{}
		for _, m := range miners {
			for k, v := range m.Blocks {
				for _, b := range v {
					// Plot ALL blocks together.
					// Some numbers will be duplicated.
					data = append(data, plotter.XY{X: float64(k), Y: float64(b.td)})
				}
			}

			scatter, err := plotter.NewScatter(data)
			if err != nil {
				panic(err)
			}
			scatter.Radius = 1
			scatter.Shape = draw.CircleGlyph{}
			scatter.Color, _ = ParseHexColor("#" + m.Address)
			p.Add(scatter)
			p.Legend.Add(m.Address, scatter)
		}

		// p.Y.Min = float64(genesisBlock.td)
		p.Save(800, 300, filename)
	}
	plotMinerTDs()

	plotMinerReorgs := func() {

		filename := filepath.Join(outDir, "miner_reorgs.png")
		p := plot.New()

		adds := plotter.XYs{}
		drops := plotter.XYs{}
		for i, m := range miners {
			i += 1
			centerMinerInterval := float64(i)
			for k, v := range m.reorgs {
				adds = append(adds, plotter.XY{X: float64(k), Y: float64(centerMinerInterval + float64(v.add)/20)})
				drops = append(drops, plotter.XY{X: float64(k), Y: float64(centerMinerInterval - float64(v.drop)/20)})
			}

			addScatter, err := plotter.NewScatter(adds)
			if err != nil {
				panic(err)
			}
			addScatter.Radius = 1
			addScatter.Shape = draw.CircleGlyph{}
			addScatter.Color = color.RGBA{R: 1, G: 255, B: 1, A: 255}
			p.Add(addScatter)

			dropScatter, err := plotter.NewScatter(drops)
			if err != nil {
				panic(err)
			}
			dropScatter.Radius = 1
			dropScatter.Shape = draw.CircleGlyph{}
			dropScatter.Color = color.RGBA{R: 255, G: 1, B: 1, A: 255}
			p.Add(dropScatter)
		}

		p.Y.Max = float64(len(miners) + 1)

		// p.Y.Min = float64(genesisBlock.td)
		p.Save(800, vg.Length(float64(len(miners)+1)*20), filename)
	}
	plotMinerReorgs()

	// plotMinerReorgMagnitudes := func() {
	// 	filename := filepath.Join("out", "miner_tds.png")
	// 	p := plot.New()
	//
	// 	data := plotter.XYs{}
	// 	for _, m := range miners {
	// 		for k, v := range m.re {
	// 			for _, b := range v {
	// 				// Plot ALL blocks together.
	// 				// Some numbers will be duplicated.
	// 				data = append(data, plotter.XY{X: float64(k), Y: float64(b.d)})
	// 			}
	// 		}
	//
	// 		scatter, err := plotter.NewScatter(data)
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 		scatter.Radius = 1
	// 		scatter.Shape = draw.CircleGlyph{}
	// 		scatter.Color, _ = ParseHexColor("#" + m.Address)
	// 		p.Add(scatter)
	// 		p.Legend.Add(m.Address, scatter)
	// 	}
	//
	// 	// p.Y.Min = float64(genesisBlock.td)
	// 	p.Save(800, 300, filename)
	// }
	// plotMinerReorgMagnitudes()

	/*
		https://superuser.com/questions/249101/how-can-i-combine-30-000-images-into-a-timelapse-movie

		ffmpeg -f image2 -r 1/5 -i img%03d.png -c:v libx264 -pix_fmt yuv420p out.mp4
		ffmpeg -f image2 -pattern_type glob -i 'time-lapse-files/*.JPG' …

	*/
	t.Log("Making movie...")
	movieCmd := exec.Command("/usr/bin/ffmpeg",
		"-f", "image2",
		"-r", "10/1", // 10 images / 1 second (Hz)
		// "-vframes", fmt.Sprintf("%d", lastHighBlock),
		"-pattern_type", "glob",
		"-i", filepath.Join(outDir, "anim", "*.png"),
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		filepath.Join(outDir, "anim", "out.mp4"),
	)
	if err := movieCmd.Run(); err != nil {
		t.Fatal(err)
	}

	/*
		https://askubuntu.com/questions/648603/how-to-create-an-animated-gif-from-mp4-video-via-command-line

		ffmpeg \
		  -i opengl-rotating-triangle.mp4 \
		  -r 15 \
		  -vf scale=512:-1 \
		  -ss 00:00:03 -to 00:00:06 \
		  opengl-rotating-triangle.gif
	*/
	t.Log("Making gif...")
	gifCmd := exec.Command("/usr/bin/ffmpeg",
		"-i", filepath.Join(outDir, "anim", "out.mp4"),
		"-r", "10",
		"-vf", "scale=512:-1",
		filepath.Join(outDir, "anim", "out.gif"),
	)
	if err := gifCmd.Run(); err != nil {
		t.Fatal(err)
	}

	animSlides, err := filepath.Glob(filepath.Join(outDir, "anim", "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range animSlides {
		if strings.Contains(f, "0420") {
			continue
		}
		os.Remove(f)
	}
}

func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		_, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
		// Double the hex digits:
		c.R *= 17
		c.G *= 17
		c.B *= 17
	default:
		err = fmt.Errorf("invalid length, must be 7 or 4")

	}
	return
}

type HashrateDistType int

const (
	HashrateDistEqual HashrateDistType = iota
	HashrateDistLongtail
)

func (t HashrateDistType) String() string {
	switch t {
	case HashrateDistEqual:
		return "equal"
	case HashrateDistLongtail:
		return "longtail"
	default:
		panic("unknown")
	}
}

func generateMinerHashrates(ty HashrateDistType, n int) []float64 {
	if n < 1 {
		panic("must have at least one miner")
	}
	if n == 1 {
		return []float64{1}
	}

	out := []float64{}

	switch ty {
	case HashrateDistLongtail:
		rem := float64(1)
		for i := 0; i < n; i++ {
			var take float64
			var share float64
			if i == 0 {
				share = float64(1) / 3
			} else {
				share = 0.6
			}
			if i != n-1 {
				take = rem * share
			}
			if take > float64(1)/3*rem {
				take = float64(1) / 3 * rem
			}
			if i == n-1 {
				take = rem
			}
			out = append(out, take)
			rem = rem - take
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i] > out[j]
		})
		return out
	case HashrateDistEqual:
		for i := 0; i < n; i++ {
			out = append(out, float64(1)/float64(n))
		}
		return out
	default:
		panic("impossible")
	}
}
