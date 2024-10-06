package prometheus

import (
	"sync/atomic"
	"time"

	dto "github.com/prometheus/client_model/go"
)

type prometheusHistogram struct {
	name string
	c    *dto.Metric
}

func (c prometheusHistogram) Reset() {
}

func (c prometheusHistogram) Update(n float64) {
	atomic.AddUint64(c.c.Histogram.SampleCount, 1)
	addFloat64(c.c.Histogram.SampleSum, n)
	for _, b := range c.c.Histogram.Bucket {
		if n > *b.UpperBound {
			continue
		}
		atomic.AddUint64(b.CumulativeCount, 1)
	}
}

func (c prometheusHistogram) UpdateDuration(n time.Time) {
	x := time.Since(n).Seconds()
	atomic.AddUint64(c.c.Histogram.SampleCount, 1)
	addFloat64(c.c.Histogram.SampleSum, x)
	for _, b := range c.c.Histogram.Bucket {
		if x > *b.UpperBound {
			continue
		}
		atomic.AddUint64(b.CumulativeCount, 1)
	}
}
