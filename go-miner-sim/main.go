package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
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
		runSimPlotting(c.name, c.minerMutation)
	}
}

// Globals
var ticksPerSecond int64 = 10
var tickSamples = ticksPerSecond * int64((time.Hour * 24).Seconds())
var networkLambda = (float64(1) / float64(13)) / float64(ticksPerSecond)
var countMiners = int64(12)
var minerNeighborRate float64 = 0.5 // 0.7
var blockReward int64 = 3

var latencySecondsDefault float64 = 2.5
var delaySecondsDefault float64 = 0 // miner hesitancy to broadcast solution

const tabsAdjustmentDenominator = int64(128) // int64(4096) <-- 4096 is the 'equilibrium' value, lower values prefer richer miners more (devaluing hashrate)
const genesisBlockTABS int64 = 10_000        // tabs starting value
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

type Blocks []*Block
type BlockTree map[int64]Blocks

type Delay struct {
	subjective int64
	material   int64
}

type Miner struct {
	Index   int64
	Address string
	Blocks  BlockTree

	HashesPerTick int64 // per tick
	Balance       int64 // Wei
	BalanceCap    int64 // Max Wei this miner will hold. Use 0 for no limit hold 'em.
	CostPerBlock  int64 // cost to miner, expended after each block win (via tx on text block)

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

func (m *Miner) mineTick() {
	parent := m.head

	// - HashesPerTick / parent.difficulty gives relative network hashrate share
	// - * m.Lambda gives relative trial share per tick
	tickR := float64(m.HashesPerTick) / float64(parent.d) * networkLambda
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

		blockDifficulty := getBlockDifficulty(parent, uncles, (s-parent.s)*ticksPerSecond)
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

// setHead sets the given block as the miner's canonical head.
// It assumes that canon-arbitration has already happened,
// and that this head block is indeed canonical and should be the best block ever.
func (m *Miner) setHead(head *Block) {
	// Should never happen, but handle the case.
	if m.head.h == head.h {
		return
	}

	doReorg := m.head.h != head.ph
	if doReorg {
		// Reorg!
		add, drop := 1, 0

		// ph is an iterated value during common ancestor finding.
		ph := head.ph

		// outer iterates backwards from the parent of the head block
		// it breaks when it finds a common ancestor
	outer:
		for i := head.i - 1; i > 0; i-- {
			for _, b := range m.Blocks[i] {
				if b.canonical && b.h == ph {
					break outer

				} else if b.canonical {
					if b.miner == m.Address {
						m.balanceAdd(-blockReward)
					}
					drop++
					b.canonical = false
				} else if !b.canonical && b.h == ph {
					if b.miner == m.Address {
						m.balanceAdd(blockReward)
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
						m.balanceAdd(-blockReward)
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
						m.balanceAdd(-blockReward)
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
		m.balanceAdd(blockReward)
	}

	headI := head.i

	m.cord <- minerEvent{
		minerI: int(m.Index),
		i:      headI,
		blocks: m.Blocks[headI],
	}
}

func (m *Miner) balanceAdd(i int64) {
	m.Balance += i
	if m.BalanceCap != 0 && m.Balance > m.BalanceCap {
		m.Balance = m.BalanceCap
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

func (d Delay) Total() int64 {
	return d.subjective + d.material
}

func (bs Blocks) Len() int {
	return len(bs)
}

func NewBlockTree() BlockTree {
	return make(map[int64]Blocks)
}

func (bt BlockTree) String() string {
	out := ""
	for i := int64(0); i < int64(len(bt)); i++ {

		out += fmt.Sprintf("n=%d ", i)
		for _, b := range bt[i] {
			out += fmt.Sprintf("[h=%s ph=%s c=%v]", b.h, b.ph, b.canonical)
		}
		out += "\n"
	}

	return out
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

// CanonicalIntervals returns canonical block intervals for a tree.
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

func (bt BlockTree) Where(condition func(*Block) bool) (blocks Blocks) {
	for _, v := range bt {
		for _, bl := range v {
			if !condition(bl) {
				continue
			}
			blocks = append(blocks, bl)
		}
	}
	return blocks
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
