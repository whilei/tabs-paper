package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/montanaflynn/stats"
	exprand "golang.org/x/exp/rand"
	"gonum.org/v1/gonum/stat/distuv"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg/draw"
)

func TestPoissonCDFNextBlock(t *testing.T) {

	interval := float64(6)
	rate := float64(1) / float64(14)
	poisson := distuv.Poisson{
		Lambda: float64(interval * rate),
		Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
	}

	t.Log("PDF", 1, poisson.Prob(2), `Within 1 second probability of seeing 2 blocks, given block rate of 1/14s (== 1 block/14s == 1/14 block / 1s)`)
	t.Log("PDF", 1, 1-poisson.CDF(2))

	sum := float64(0)
	for i := 2; i <= 99; i++ {
		sum += poisson.Prob(float64(i))
	}
	t.Log("PPDF", sum)

	// sum := float64(0)
	// for i := 2; i <= 99; i++ {
	// 	sum += poisson.Prob(float64(i))
	// }
	// t.Log("PPDF", sum)

	p := plot.New()
	data := plotter.XYs{}
	for i := 0; i < 99; i++ {
		interval = float64(i)
		rate = float64(1) / float64(14)
		poisson = distuv.Poisson{
			Lambda: float64(interval * rate),
			Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
		}
		data = append(data, plotter.XY{X: float64(i), Y: float64(1) - float64(poisson.CDF(0))})
		// t.Log("CDF", 0, 1-poisson.CDF(0))
	}

	scatter, _ := plotter.NewScatter(data)
	scatter.Radius = 2
	scatter.Shape = draw.CircleGlyph{}
	p.Add(scatter)

	p.Title.Padding = 16
	p.Legend.Top = true
	p.Legend.Padding = 8
	p.Legend.YOffs = -16
	p.Legend.XOffs = -16
	p.Title.Text = "Probability of Next Block over Time (1-CDF(0),lambda=x/14seconds)"
	p.X.Label.Text = "Interval (Seconds)"
	p.Y.Label.Text = "Probability of At Least One Block"

	filename := filepath.Join("out", "vis_poisson_cdf_next_in_interval.png")
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}
}

func TestPoissonIntervals(t *testing.T) {

	numSamples := 3600 * 24
	// latency := float64(3)

	p := plot.New()
	data := plotter.XYs{}

	last := 0
	rs := []float64{}
	intervals := []float64{}
	forks := []float64{}
	forksTally := 0
	for i := 1; i < numSamples; i++ {
		interval := float64(1)
		rate := float64(1) / float64(14) // constant
		poisson := distuv.Poisson{
			Lambda: float64(interval * rate),
			Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
		}
		// sum := float64(0)
		// for m := float64(1); m < 100; m++ {
		// 	sum += poisson.Prob(m)
		// }

		// data = append(data, plotter.XY{X: float64(i), Y: sum})
		// data = append(data, plotter.XY{X: float64(i), Y: poisson.Prob(1)})
		r := poisson.Rand()
		rs = append(rs, r)
		if r > 0 {
			gotInterval := float64(i) - float64(last)
			// if rand.Float64() < float64(1) / 8 {
			// 	// nothing (same miner)
			// } else {
			// 	gotInterval += latency
			// }

			intervals = append(intervals, gotInterval)
			last = i
		}
		if r > 1 {
			forks = append(forks, r)
			forksTally++
		}
		data = append(data, plotter.XY{X: float64(i), Y: r})
		// t.Log("CDF", 0, 1-poisson.CDF(0))
	}

	t.Log(printStats("Rs", rs))
	t.Log(printStats("INTERVALS", intervals))
	t.Logf("FORKS rate=%0.4f tally=%d", float64(forksTally)/float64(len(intervals)), forksTally)
	t.Log(printStats("FORKS", forks))

	scatter, _ := plotter.NewScatter(data)
	scatter.Radius = 1
	scatter.Shape = draw.CircleGlyph{}
	p.Add(scatter)

	p.Title.Padding = 16
	p.Legend.Top = true
	p.Legend.Padding = 8
	p.Legend.YOffs = -16
	p.Legend.XOffs = -16
	p.Title.Text = "Random Samples of Poisson Event Occurrences (lambda=1/14seconds)"
	p.X.Label.Text = "Sample Number"
	p.Y.Label.Text = "K occurrences (number of block events)"

	filename := filepath.Join("out", fmt.Sprintf("vis_poisson_samples_events_%d.png", numSamples))
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	intervalBuckets := map[float64]int{}
	for _, interval := range intervals {
		intervalBuckets[interval]++
	}
	histData := plotter.XYs{}
	for k, v := range intervalBuckets {
		histData = append(histData, plotter.XY{X: float64(k), Y: float64(v)})
	}

	p = plot.New()
	p.Title.Text = fmt.Sprintf("Histogram of %d Sampled Poisson Distributed (lambda=1/14seconds) Occurrences", numSamples)
	p.X.Label.Text = "Samples Taken Between >=1 Occurrence"
	p.Y.Label.Text = "Number of Occurrences"
	hist, _ := plotter.NewHistogram(histData, len(intervalBuckets))
	p.Add(hist)
	filename = filepath.Join("out", "vis_poisson_samples_eventintervals_hist.png")
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	t.Logf("Forks: %0.2f", float64(len(forks))/float64(len(intervals)))

}

func TestPoissonIntervals_Latency1(t *testing.T) {
	numSamples := 3600 * 24
	latency := float64(3)

	p := plot.New()
	data := plotter.XYs{}

	last := 0
	rs := []float64{}
	intervals := []float64{}
	forks := []float64{}
	forksTally := 0
	for i := 1; i < numSamples; i++ {
		interval := float64(1)
		rate := float64(1) / float64(14) // constant
		poisson := distuv.Poisson{
			Lambda: float64(interval * rate),
			Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
		}
		// sum := float64(0)
		// for m := float64(1); m < 100; m++ {
		// 	sum += poisson.Prob(m)
		// }

		// data = append(data, plotter.XY{X: float64(i), Y: sum})
		// data = append(data, plotter.XY{X: float64(i), Y: poisson.Prob(1)})
		r := poisson.Rand()
		rs = append(rs, r)
		if r > 0 {
			gotInterval := float64(i) - float64(last)
			// if rand.Float64() < float64(1) / 8 {
			// nothing (same miner)
			// } else {
			gotInterval += latency
			// }

			intervals = append(intervals, gotInterval)
			last = i
		}
		if r > 1 {
			forks = append(forks, r)
			forksTally++
		}
		data = append(data, plotter.XY{X: float64(i), Y: r})
		// t.Log("CDF", 0, 1-poisson.CDF(0))
	}

	t.Log(printStats("Rs", rs))
	t.Log(printStats("INTERVALS", intervals))
	t.Logf("FORKS rate=%0.4f tally=%d", float64(forksTally)/float64(len(intervals)), forksTally)
	t.Log(printStats("FORKS", forks))

	scatter, _ := plotter.NewScatter(data)
	scatter.Radius = 1
	scatter.Shape = draw.CircleGlyph{}
	p.Add(scatter)

	p.Title.Padding = 16
	p.Legend.Top = true
	p.Legend.Padding = 8
	p.Legend.YOffs = -16
	p.Legend.XOffs = -16
	p.Title.Text = "Random Samples of Poisson Event Occurrences (lambda=1/14seconds), With Latency"
	p.X.Label.Text = "Sample Number"
	p.Y.Label.Text = "K occurrences (number of block events)"

	filename := filepath.Join("out", fmt.Sprintf("vis_poisson_samples_events_latencynaive_%d.png", numSamples))
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	intervalBuckets := map[float64]int{}
	for _, interval := range intervals {
		intervalBuckets[interval]++
	}
	histData := plotter.XYs{}
	for k, v := range intervalBuckets {
		histData = append(histData, plotter.XY{X: float64(k), Y: float64(v)})
	}

	p = plot.New()
	p.Title.Text = fmt.Sprintf("Histogram of %d Sampled Poisson Distributed (lambda=1/14seconds) Occurrences, with Latency", numSamples)
	p.X.Label.Text = "Samples Taken Between >=1 Occurrence"
	p.Y.Label.Text = "Number of Occurrences"
	hist, _ := plotter.NewHistogram(histData, len(intervalBuckets))
	p.Add(hist)
	filename = filepath.Join("out", "vis_poisson_samples_eventintervals_latencynaive_hist.png")
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	t.Logf("Forks: %0.2f", float64(len(forks))/float64(len(intervals)))

}

func TestPoissonIntervals_Latency2(t *testing.T) {
	numSamples := 3600 * 24
	latency := float64(3)

	p := plot.New()
	data := plotter.XYs{}

	last := 0
	rs := []float64{}
	intervals := []float64{}
	forks := []float64{}
	forksTally := 0
	for i := 1; i < numSamples; i++ {
		interval := float64(1)
		rate := float64(1) / float64(14) // constant
		poisson := distuv.Poisson{
			Lambda: float64(interval * rate),
			Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
		}
		// sum := float64(0)
		// for m := float64(1); m < 100; m++ {
		// 	sum += poisson.Prob(m)
		// }

		// data = append(data, plotter.XY{X: float64(i), Y: sum})
		// data = append(data, plotter.XY{X: float64(i), Y: poisson.Prob(1)})
		r := poisson.Rand()
		rs = append(rs, r)
		if r > 0 {
			gotInterval := float64(i) - float64(last)
			if rand.Float64() < float64(1)/8 {
				// nothing (same miner)
			} else {
				gotInterval += latency
				// gotInterval = gotInterval * (1 + (1/8))
			}

			intervals = append(intervals, gotInterval)
			last = i
		}
		if r > 1 {
			forks = append(forks, r)
			forksTally++
		}
		data = append(data, plotter.XY{X: float64(i), Y: r})
		// t.Log("CDF", 0, 1-poisson.CDF(0))
	}

	t.Log(printStats("Rs", rs))
	t.Log(printStats("INTERVALS", intervals))
	t.Logf("FORKS rate=%0.4f tally=%d", float64(forksTally)/float64(len(intervals)), forksTally)
	t.Log(printStats("FORKS", forks))

	scatter, _ := plotter.NewScatter(data)
	scatter.Radius = 1
	scatter.Shape = draw.CircleGlyph{}
	p.Add(scatter)

	p.Title.Padding = 16
	p.Legend.Top = true
	p.Legend.Padding = 8
	p.Legend.YOffs = -16
	p.Legend.XOffs = -16
	p.Title.Text = "Random Samples of Poisson Event Occurrences (lambda=1/14seconds), With Latency"
	p.X.Label.Text = "Sample Number"
	p.Y.Label.Text = "K occurrences (number of block events)"

	filename := filepath.Join("out", fmt.Sprintf("vis_poisson_samples_events_latencysamesame_%d.png", numSamples))
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	intervalBuckets := map[float64]int{}
	for _, interval := range intervals {
		intervalBuckets[interval]++
	}
	histData := plotter.XYs{}
	for k, v := range intervalBuckets {
		histData = append(histData, plotter.XY{X: float64(k), Y: float64(v)})
	}

	p = plot.New()
	p.Title.Text = fmt.Sprintf("Histogram of %d Sampled Poisson Distributed (lambda=1/14seconds) Occurrences, with Latency", numSamples)
	p.X.Label.Text = "Samples Taken Between >=1 Occurrence"
	p.Y.Label.Text = "Number of Occurrences"
	hist, _ := plotter.NewHistogram(histData, len(intervalBuckets))
	p.Add(hist)
	filename = filepath.Join("out", "vis_poisson_samples_eventintervals_latencysamesame_hist.png")
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	t.Logf("Forks: %0.2f", float64(len(forks))/float64(len(intervals)))

}


func TestPoissonIntervals_Latency3_Strategic(t *testing.T) {
	numSamples := 3600 * 24
	latency := float64(3)

	p := plot.New()
	data := plotter.XYs{}

	last := 0
	rs := []float64{}
	intervals := []float64{}
	forks := []float64{}
	forksTally := 0
	for i := 1; i < numSamples; i++ {
		interval := float64(1)
		rate := float64(1) / float64(14) // constant
		poisson := distuv.Poisson{
			Lambda: float64(interval * rate),
			Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
		}
		// sum := float64(0)
		// for m := float64(1); m < 100; m++ {
		// 	sum += poisson.Prob(m)
		// }

		// data = append(data, plotter.XY{X: float64(i), Y: sum})
		// data = append(data, plotter.XY{X: float64(i), Y: poisson.Prob(1)})
		r := poisson.Rand()
		rs = append(rs, r)
		if r > 0 {
			gotInterval := float64(i) - float64(last)
			if rand.Float64() < float64(1)/8 {
				// nothing (same miner)
			} else {
				mean, err := stats.Median(intervals)
				if err != nil || len(intervals) == 0 {
					mean = float64(14)
				}
				gotInterval += float64(rand.Intn(int(latency))) + 1

				gotInterval += float64(1)/float64(8) * mean
				// gotInterval = gotInterval * (1 + (1/8))
			}

			intervals = append(intervals, gotInterval)
			last = i
		}
		if r > 1 {
			forks = append(forks, r)
			forksTally++
		}
		data = append(data, plotter.XY{X: float64(i), Y: r})
		// t.Log("CDF", 0, 1-poisson.CDF(0))
	}

	t.Log(printStats("Rs", rs))
	t.Log(printStats("INTERVALS", intervals))
	t.Logf("FORKS rate=%0.4f tally=%d", float64(forksTally)/float64(len(intervals)), forksTally)
	t.Log(printStats("FORKS", forks))

	scatter, _ := plotter.NewScatter(data)
	scatter.Radius = 1
	scatter.Shape = draw.CircleGlyph{}
	p.Add(scatter)

	p.Title.Padding = 16
	p.Legend.Top = true
	p.Legend.Padding = 8
	p.Legend.YOffs = -16
	p.Legend.XOffs = -16
	p.Title.Text = "Random Samples of Poisson Event Occurrences (lambda=1/14seconds), With Strategic Latency"
	p.X.Label.Text = "Sample Number"
	p.Y.Label.Text = "K occurrences (number of block events)"

	filename := filepath.Join("out", fmt.Sprintf("vis_poisson_samples_events_latencysamesamestrat_%d.png", numSamples))
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	intervalBuckets := map[float64]int{}
	for _, interval := range intervals {
		ival := int(interval)
		intervalBuckets[float64(ival)]++
	}
	histData := plotter.XYs{}
	for k, v := range intervalBuckets {
		histData = append(histData, plotter.XY{X: float64(k), Y: float64(v)})
	}

	p = plot.New()
	p.Title.Text = fmt.Sprintf("Histogram of %d Sampled Poisson Distributed (lambda=1/14seconds) Occurrences, with Strategic Latency", numSamples)
	p.X.Label.Text = "Samples Taken Between >=1 Occurrence"
	p.Y.Label.Text = "Number of Occurrences"
	hist, _ := plotter.NewHistogram(histData, len(intervalBuckets))
	p.Add(hist)
	filename = filepath.Join("out", "vis_poisson_samples_eventintervals_latencysamesamestrat_hist.png")
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	t.Logf("Forks: %0.2f", float64(len(forks))/float64(len(intervals)))

}

func TestPoissonIntervals2(t *testing.T) {

	numSamples := 3600 * 24 * 100

	p := plot.New()
	data := plotter.XYs{}

	last := 0
	rs := []float64{}
	intervals := []float64{}
	forks := []float64{}
	for i := 1; i < numSamples; i++ {
		interval := float64(1)
		rate := float64(1) / float64(14*100) // constant
		poisson := distuv.Poisson{
			Lambda: float64(interval * rate),
			Src:    exprand.NewSource(uint64(time.Now().UnixNano())),
		}
		// sum := float64(0)
		// for m := float64(1); m < 100; m++ {
		// 	sum += poisson.Prob(m)
		// }

		// data = append(data, plotter.XY{X: float64(i), Y: sum})
		// data = append(data, plotter.XY{X: float64(i), Y: poisson.Prob(1)})
		r := poisson.Rand()
		rs = append(rs, r)
		if r > 0 {
			intervals = append(intervals, float64(i)-float64(last))
			last = i
		}
		if r > 1 {
			forks = append(forks, r)
		}
		data = append(data, plotter.XY{X: float64(i), Y: r})
		// t.Log("CDF", 0, 1-poisson.CDF(0))
	}

	t.Log(printStats("Rs", rs))
	t.Log(printStats("INTERVALS", intervals))
	t.Log(printStats("FORKS", forks))

	scatter, _ := plotter.NewScatter(data)
	scatter.Radius = 1
	scatter.Shape = draw.CircleGlyph{}
	p.Add(scatter)

	p.Title.Text = "Random Samples of Poisson Event Occurrences (lambda=1/14seconds)"
	p.X.Label.Text = "Sample Number"
	p.Y.Label.Text = "K occurrences (number of block events)"

	filename := filepath.Join("out", fmt.Sprintf("vis_poisson_samples_events_%d.png", numSamples))
	if err := p.Save(800, 300, filename); err != nil {
		panic(err)
	}

	intervalBuckets := map[float64]int{}
	for _, interval := range intervals {
		intervalBuckets[interval]++
	}
	histData := plotter.XYs{}
	for k, v := range intervalBuckets {
		histData = append(histData, plotter.XY{X: float64(k), Y: float64(v)})
	}

	p = plot.New()
	p.Title.Padding = 16
	p.Legend.Top = true
	p.Legend.Padding = 8
	p.Legend.YOffs = -16
	p.Legend.XOffs = -16
	p.Title.Text = fmt.Sprintf("Histogram of %d Sampled Poisson Distributed (lambda=1/14seconds) Occurrences", numSamples)
	p.X.Label.Text = "Samples Taken Between >=1 Occurrence"
	p.Y.Label.Text = "Number of Occurrences"
	hist, _ := plotter.NewHistogram(histData, len(intervalBuckets))
	p.Add(hist)
	filename = filepath.Join(os.TempDir(), "plots", "hfctabs", "test5.png")
	if err := p.Save(800, 600, filename); err != nil {
		panic(err)
	}

	t.Logf("Forks: %0.2f", float64(len(forks))/float64(len(intervals)))
}
