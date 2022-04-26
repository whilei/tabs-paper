package main

import (
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/fogleman/gg"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/mazznoer/colorgrad"
	"github.com/montanaflynn/stats"
	"golang.org/x/image/colornames"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

func runSimPlotting(name string, mut func(m *Miner)) {

	log.Println("Running", name)

	outDir := filepath.Join("out", name)
	os.MkdirAll(outDir, os.ModePerm)

	os.RemoveAll(filepath.Join(outDir, "anim"))
	os.MkdirAll(filepath.Join(outDir, "anim"), os.ModePerm)

	// os.RemoveAll(filepath.Join(outDir, "montage"))
	// os.MkdirAll(filepath.Join(outDir, "montage"), os.ModePerm)

	miners := []*Miner{}
	minerEvents := make(chan minerEvent)
	blockRowsN := 150

	hashrates := generateMinerHashrates(HashrateDistLongtail, int(countMiners))
	deriveMinerRelativeDifficultyHashes := func(genesisD int64, r float64) int64 {
		return int64(float64(genesisD) * r)
	}

	// We use relative hashrate as a proxy for balance;
	// more mining capital :: more currency capital.
	deriveMinerStartingBalance := func(genesisTABS int64, minerHashrate float64) int64 {
		supply := genesisTABS * countMiners
		return int64((float64(supply) * minerHashrate))
	}

	lastColor := colorful.Color{}
	grad := colorgrad.Viridis()

	for i := int64(0); i < countMiners; i++ {

		// set up their starting view of the chain
		bt := NewBlockTree()
		bt.AppendBlockByNumber(genesisBlock)

		// set up the miner

		minerStartingBalance := deriveMinerStartingBalance(genesisBlock.tabs, hashrates[i])
		// minerStartingBalance := deriveMinerStartingBalance(genesisBlock.tabs, hashrates[countMiners - 1 - i]) // backwards
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
			Index:         i,
			Address:       minerName, // avoid collisions
			HashesPerTick: hashes,
			Balance:       minerStartingBalance,
			// BalanceCap:               minerStartingBalance,
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
			// log.Println("minerEvent", event)

			// log.Println("here", 1)
			// log.Printlnf("event: %v", event)

			// ctx.DrawCircle(rand.Float64()*float64(c.Width), rand.Float64()*float64(c.Height), 10)

			xW := (c.Width() - (2 * marginX)) / int(countMiners)
			x := event.minerI*xW + marginX

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
				// log.Printlnf("push")
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
						log.Println("bad color", err.Error())
						panic("test")
					}
					c.SetColor(clr)
				}

				realX := float64(x)
				realX += float64(ib) * float64(xW/nblocks)

				rectMargin := float64(0)

				rectX, rectY := realX+rectMargin, float64(y)+rectMargin
				rectW, rectH := float64(xW/nblocks)-(2*rectMargin), float64(yH)-(2*rectMargin)

				// log.Println("here.ib", ib, b == nil, b.miner, clr)
				// log.Printlnf("x=%d y=%d width=%v height=%v b=%d/%d", x, y, xW, yH, ib, nblocks)
				c.DrawRectangle(rectX, rectY, rectW, rectH)
				c.Fill()
				c.Stroke()
				c.Pop()
				// log.Printlnf("pop")
			}

			// log.Println("here", 2)

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

			imgBaseName := fmt.Sprintf("%04d_f.png", nextHighBlock)
			if err := c.SavePNG(filepath.Join(outDir, "anim", imgBaseName)); err != nil {
				log.Fatalln("save png errored", err)
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

	log.Println("RESULTS", name)

	for i, m := range miners {
		kMean, _ := stats.Mean(m.Blocks.Ks())
		kMed, _ := stats.Median(m.Blocks.Ks())
		kMode, _ := stats.Mode(m.Blocks.Ks())

		intervalsMean, _ := stats.Mean(m.Blocks.CanonicalIntervals())
		intervalsMean = intervalsMean / float64(ticksPerSecond)
		difficultiesMean, _ := stats.Mean(m.Blocks.CanonicalDifficulties())

		reorgMagsMean, _ := stats.Mean(m.reorgMagnitudes())

		wins := m.Blocks.Where(func(b *Block) bool {
			return b.canonical && b.miner == m.Address
		}).Len()

		// log.Printlnf(`a=%s c=%s hr=%0.2f h/t=%d head.i=%d head.tabs=%d k_mean=%0.3f k_med=%0.3f k_mode=%v intervals_mean=%0.3fs d_mean.rel=%0.3f balance=%d objective_decs=%0.3f reorgs.mag_mean=%0.3f`,
		log.Printf(`a=%s c=%s hr=%0.2f winr=%0.3f wins=%d head.i=%d head.tabs=%d k_mean=%0.3f k_med=%0.3f k_mode=%v intervals_mean=%0.3fs d_mean.rel=%0.3f balance=%d objective_decs=%0.3f reorgs.mag_mean=%0.3f\n`,
			m.Address, m.ConsensusAlgorithm, hashrates[i], float64(wins)/float64(m.head.i), wins, /* m.HashesPerTick, */
			m.head.i, m.head.tabs,
			kMean, kMed, kMode,
			intervalsMean, difficultiesMean/float64(genesisBlock.d),
			m.Balance,
			float64(m.ConsensusObjectiveArbitrations)/float64(m.ConsensusArbitrations),
			reorgMagsMean)
	}

	log.Println("Making plots...")

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
	log.Println("Making movie...")
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
		log.Fatalln(err)
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
	log.Println("Making gif...")
	gifCmd := exec.Command("/usr/bin/ffmpeg",
		"-i", filepath.Join(outDir, "anim", "out.mp4"),
		"-r", "10",
		"-vf", "scale=512:-1",
		filepath.Join(outDir, "anim", "out.gif"),
	)
	if err := gifCmd.Run(); err != nil {
		log.Fatalln(err)
	}

	animSlides, err := filepath.Glob(filepath.Join(outDir, "anim", "*.png"))
	if err != nil {
		log.Fatalln(err)
	}
	for _, f := range animSlides {
		// 			imgBaseName := fmt.Sprintf("%04d_f.png", nextHighBlock)
		base := filepath.Base(f)
		numStr := base[:4]
		i, err := strconv.Atoi(numStr)
		if err != nil {
			log.Fatalln(err)
		}
		if i%blockRowsN == 0 {
			// do nothing; this is a unique frame and could be used for composition in a montage
		} else {
			os.Remove(f)
		}
	}
}
