package main

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/image/colornames"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg/draw"
)

type DifficultyModTickerInterval struct{}

func (mt DifficultyModTickerInterval) Ticks(min, max float64) []plot.Tick {
	out := []plot.Tick{}
	for i := min; i < max; i++ {
		if i == min || i == max || math.Mod(i, 9) == 0 {
			out = append(out, plot.Tick{
				Value: i,
				Label: fmt.Sprintf("%0.0f", i),
			})
		}
	}
	return out
}

type DifficultyModTickerAdjustment struct {
	uniqs []float64
}

func (mt DifficultyModTickerAdjustment) Ticks(min, max float64) []plot.Tick {
	out := []plot.Tick{}
	for _, v := range mt.uniqs {
		out = append(out, plot.Tick{
			Value: v,
			Label: fmt.Sprintf("%0.5f", v),
		})
	}
	return out
}

func TestPlotDifficultyAdjustments(t *testing.T) {
	config := params.AllEthashProtocolChanges
	genesis := core.DefaultGenesisBlock()
	parentBlock := genesis.ToBlock(rawdb.NewMemoryDatabase())

	parentBlockUncles := core.DefaultGenesisBlock().ToBlock(rawdb.NewMemoryDatabase())
	uncle := types.CopyHeader(parentBlockUncles.Header())
	parentBlockUncles = types.NewBlock(parentBlockUncles.Header(), nil, []*types.Header{uncle}, nil, nil)

	p, err := plot.New()
	if err != nil {
		t.Fatal(err)
	}
	p.Title.Text = "Ethash Difficulty Adjustments"
	p.Y.Label.Text = "Relative Adjusted Difficulty"
	p.X.Label.Text = "Time Interval of Block from Parent (field: timestamp)"

	data := plotter.XYs{}
	dataUncles := plotter.XYs{}
	uniqs := []float64{} // for labeling y axis ticks

	parentDifficultyF := new(big.Float).SetInt(parentBlock.Difficulty())

	for i := uint64(1); i < 99; i++ {
		// No uncles
		difficultyF := new(big.Float).SetInt(ethash.CalcDifficulty(config, genesis.Timestamp+i, parentBlock.Header()))

		ratio := difficultyF.Quo(difficultyF, parentDifficultyF)
		fl, _ := ratio.Float64()

		data = append(data, plotter.XY{X: float64(i), Y: fl})

		t.Log(i, fl)

		// Uncles
		difficultyF = new(big.Float).SetInt(ethash.CalcDifficulty(config, genesis.Timestamp+i, parentBlockUncles.Header()))

		ratio = difficultyF.Quo(difficultyF, parentDifficultyF)
		fl, _ = ratio.Float64()

		dataUncles = append(dataUncles, plotter.XY{X: float64(i), Y: fl})

		// Collect uniq adjustment values for the axis tick labels
		uniq := true
		for _, v := range uniqs {
			if v == fl {
				uniq = false
				break
			}
		}
		if uniq {
			uniqs = append(uniqs, fl)
		}
	}

	plot, err := plotter.NewScatter(data)
	if err != nil {
		t.Fatal(err)
	}
	plot.GlyphStyle.Shape = draw.CrossGlyph{}
	plot.GlyphStyle.Radius = 3
	plot.GlyphStyle.Color = colornames.Blue
	p.Add(plot)

	plotUncles, err := plotter.NewScatter(dataUncles)
	if err != nil {
		t.Fatal(err)
	}
	plotUncles.GlyphStyle.Shape = draw.PlusGlyph{}
	plotUncles.GlyphStyle.Radius = 3
	plotUncles.GlyphStyle.Color = colornames.Red
	p.Add(plotUncles)

	p.Legend.Add("no uncles", plot)
	p.Legend.Add("uncles", plotUncles)
	p.Legend.Top = true

	p.X.Tick.Marker = DifficultyModTickerInterval{}
	p.X.Tick.Length = 0

	p.Y.Tick.Marker = DifficultyModTickerAdjustment{uniqs: uniqs}
	p.Y.Tick.Length = 0

	if err := p.Save(800, 600, "ethash_difficulty_adjustment.png"); err != nil {
		t.Fatal(err)
	}
}

func TestPlotCanonScores_Incumbent(t *testing.T) {
	config := params.AllEthashProtocolChanges
	genesis := core.DefaultGenesisBlock()
	parentBlock := genesis.ToBlock(rawdb.NewMemoryDatabase())

	parentBlockUncles := core.DefaultGenesisBlock().ToBlock(rawdb.NewMemoryDatabase())
	uncle := types.CopyHeader(parentBlockUncles.Header())
	parentBlockUncles = types.NewBlock(parentBlockUncles.Header(), nil, []*types.Header{uncle}, nil, nil)

	p, err := plot.New()
	if err != nil {
		t.Fatal(err)
	}
	p.Title.Text = "Ethash/GHOST Parent-Relative Canon Scores over Block Interval"
	p.Y.Label.Text = "Relative Adjusted Difficulty"
	p.X.Label.Text = "Time Interval of Block from Parent (field: timestamp)"

	data := plotter.XYs{}
	dataUncles := plotter.XYs{}
	uniqs := []float64{} // for labeling y axis ticks

	parentDifficultyF := new(big.Float).SetInt(parentBlock.Difficulty())

	for i := uint64(1); i < 99; i++ {
		// No uncles
		difficultyF := new(big.Float).SetInt(ethash.CalcDifficulty(config, genesis.Timestamp+i, parentBlock.Header()))

		ratio := difficultyF.Quo(difficultyF, parentDifficultyF)
		fl, _ := ratio.Float64()

		data = append(data, plotter.XY{X: float64(i), Y: fl})

		t.Log(i, fl)

		// Uncles
		difficultyF = new(big.Float).SetInt(ethash.CalcDifficulty(config, genesis.Timestamp+i, parentBlockUncles.Header()))

		ratio = difficultyF.Quo(difficultyF, parentDifficultyF)
		fl, _ = ratio.Float64()

		dataUncles = append(dataUncles, plotter.XY{X: float64(i), Y: fl})

		// Collect uniq adjustment values for the axis tick labels
		uniq := true
		for _, v := range uniqs {
			if v == fl {
				uniq = false
				break
			}
		}
		if uniq {
			uniqs = append(uniqs, fl)
		}
	}

	plot, err := plotter.NewScatter(data)
	if err != nil {
		t.Fatal(err)
	}
	plot.GlyphStyle.Shape = draw.CrossGlyph{}
	plot.GlyphStyle.Radius = 3
	plot.GlyphStyle.Color = colornames.Blue
	p.Add(plot)

	plotUncles, err := plotter.NewScatter(dataUncles)
	if err != nil {
		t.Fatal(err)
	}
	plotUncles.GlyphStyle.Shape = draw.PlusGlyph{}
	plotUncles.GlyphStyle.Radius = 3
	plotUncles.GlyphStyle.Color = colornames.Red
	p.Add(plotUncles)

	p.Legend.Add("no uncles", plot)
	p.Legend.Add("uncles", plotUncles)
	p.Legend.Top = true

	p.X.Tick.Marker = DifficultyModTickerInterval{}
	p.X.Tick.Length = 0

	p.Y.Tick.Marker = DifficultyModTickerAdjustment{uniqs: uniqs}
	p.Y.Tick.Length = 0

	if err := p.Save(800, 600, "ethash_canon_scores.png"); err != nil {
		t.Fatal(err)
	}
}

func TestPlotCanonScores_TDTABS(t *testing.T) {
	config := params.AllEthashProtocolChanges
	genesis := core.DefaultGenesisBlock()
	parentBlock := genesis.ToBlock(rawdb.NewMemoryDatabase())

	parentBlockUncles := core.DefaultGenesisBlock().ToBlock(rawdb.NewMemoryDatabase())
	uncle := types.CopyHeader(parentBlockUncles.Header())
	parentBlockUncles = types.NewBlock(parentBlockUncles.Header(), nil, []*types.Header{uncle}, nil, nil)

	var tabsAdjustmentDenominator float64 = 2048 * 2
	// var tabsAdjustmentDenominator float64 = 128

	p, err := plot.New()
	if err != nil {
		t.Fatal(err)
	}
	p.Title.Text = fmt.Sprintf("TDTABS Parent-Relative Canon Scores over Block Interval (r=%d)", int(tabsAdjustmentDenominator))
	p.Y.Label.Text = "Relative Adjusted Difficulty"
	p.X.Label.Text = "Time Interval of Block from Parent (field: timestamp)"

	// matrix: uncles y|n :: tabs up|dn
	dataDn := plotter.XYs{}
	dataUp := plotter.XYs{}
	dataSame := plotter.XYs{}
	dataUnclesDn := plotter.XYs{}
	dataUnclesUp := plotter.XYs{}
	dataUnclesSame := plotter.XYs{}

	uniqs := []float64{} // for labeling y axis ticks

	parentDifficultyF := new(big.Float).SetInt(parentBlock.Difficulty())
	parentDifficultyf, _ := parentDifficultyF.Float64()
	var parentTABSf float64 = 128 * params.Ether // This is the proposed minimum value.

	// The parent canon score will be compared against the potential canon score outcomes for its child.
	// This is: atomic CS = TD * TABS
	parentCanonScore := float64(parentTABSf * parentDifficultyf)

	for i := uint64(1); i < 99; i++ {

		// Iterate through the set [-1,0,1] to represent the potential TABS adjustment outcomes.
		// As proposed these values correspond to the condition of whether the current block's TAB
		// exceeds, ties, or is lesser than the parent block's TABS.
		// Ties are not charted because they will only happen extremely rarely.
		for _, j := range []float64{-1, 0, 1} {
			// This program uses floats for convenience.
			// The actual implementation will need to use only integers.
			tabs := parentTABSf * (j + tabsAdjustmentDenominator) / tabsAdjustmentDenominator

			// No uncles
			difficulty := ethash.CalcDifficulty(config, genesis.Timestamp+i, parentBlock.Header())
			difficultyF := new(big.Float).SetInt(difficulty)
			difficultyf, _ := difficultyF.Float64()

			canonScore := tabs * difficultyf

			fl := canonScore / parentCanonScore

			if j < 0 {
				dataDn = append(dataDn, plotter.XY{X: float64(i), Y: fl})
			} else if j > 0 {
				dataUp = append(dataUp, plotter.XY{X: float64(i), Y: fl})
			} else {
				dataSame = append(dataSame, plotter.XY{X: float64(i), Y: fl})
			}

			t.Log(i, fl)

			// // Uncles
			difficulty = ethash.CalcDifficulty(config, genesis.Timestamp+i, parentBlockUncles.Header())
			difficultyF = new(big.Float).SetInt(difficulty)
			difficultyf, _ = difficultyF.Float64()

			canonScore = tabs * difficultyf

			fl = canonScore / parentCanonScore

			if j < 0 {
				dataUnclesDn = append(dataUnclesDn, plotter.XY{X: float64(i), Y: fl})
			} else if j > 0 {
				dataUnclesUp = append(dataUnclesUp, plotter.XY{X: float64(i), Y: fl})
			} else {
				dataUnclesSame = append(dataUnclesSame, plotter.XY{X: float64(i), Y: fl})
			}

			// dataUnclesDn = append(dataUnclesDn, plotter.XY{X: float64(i), Y: fl})

			// Collect uniq adjustment values for the axis tick labels
			uniq := true
			for _, v := range uniqs {
				if v == fl {
					uniq = false
					break
				}
			}
			if uniq {
				uniqs = append(uniqs, fl)
			}
		}
	}

	plot, err := plotter.NewScatter(dataDn)
	if err != nil {
		t.Fatal(err)
	}
	plot.GlyphStyle.Shape = draw.CrossGlyph{}

	plot.GlyphStyle.Radius = 3
	plot.GlyphStyle.Color = colornames.Blue
	p.Add(plot)

	plotUp, err := plotter.NewScatter(dataUp)
	if err != nil {
		t.Fatal(err)
	}
	plotUp.GlyphStyle.Shape = draw.PlusGlyph{}

	plotUp.GlyphStyle.Radius = 3
	plotUp.GlyphStyle.Color = colornames.Blue
	p.Add(plotUp)

	plotSame, err := plotter.NewScatter(dataSame)
	if err != nil {
		t.Fatal(err)
	}
	plotSame.GlyphStyle.Shape = draw.CrossGlyph{}
	plotSame.GlyphStyle.Radius = 2
	plotSame.GlyphStyle.Color = colornames.Gray
	p.Add(plotSame)

	plotUnclesDn, err := plotter.NewScatter(dataUnclesDn)
	if err != nil {
		t.Fatal(err)
	}
	plotUnclesDn.GlyphStyle.Shape = draw.CrossGlyph{}
	plotUnclesDn.GlyphStyle.Radius = 3
	plotUnclesDn.GlyphStyle.Color = colornames.Red
	p.Add(plotUnclesDn)

	plotUnclesUp, err := plotter.NewScatter(dataUnclesUp)
	if err != nil {
		t.Fatal(err)
	}
	plotUnclesUp.GlyphStyle.Shape = draw.PlusGlyph{}
	plotUnclesUp.GlyphStyle.Radius = 3
	plotUnclesUp.GlyphStyle.Color = colornames.Red
	p.Add(plotUnclesUp)

	plotUnclesSame, err := plotter.NewScatter(dataUnclesSame)
	if err != nil {
		t.Fatal(err)
	}
	plotUnclesSame.GlyphStyle.Shape = draw.PlusGlyph{}
	plotUnclesSame.GlyphStyle.Radius = 2
	plotUnclesSame.GlyphStyle.Color = colornames.Gray
	p.Add(plotUnclesSame)

	p.Legend.Add("no uncles, tabs falls", plot)
	p.Legend.Add("no uncles, tabs rises", plotUp)
	p.Legend.Add("no uncles, tabs same", plotSame)
	p.Legend.Add("uncles, tabs falls", plotUnclesDn)
	p.Legend.Add("uncles, tabs rises", plotUnclesUp)
	p.Legend.Add("uncles, tabs same", plotUnclesSame)
	p.Legend.Top = true

	p.X.Tick.Marker = DifficultyModTickerInterval{}
	p.X.Tick.Length = 0

	p.Y.Tick.Marker = DifficultyModTickerAdjustment{uniqs: uniqs}
	p.Y.Tick.Length = 0

	if err := p.Save(800, 600, "tdtabs_canon_scores.png"); err != nil {
		t.Fatal(err)
	}
}
