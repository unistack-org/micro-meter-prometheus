package prometheus

import (
	"sync/atomic"
	"time"

	dto "github.com/prometheus/client_model/go"
)

type prometheusSummary struct {
	name string
	c    *dto.Metric
}

func (c prometheusSummary) Update(n float64) {
	atomic.AddUint64(c.c.Summary.SampleCount, 1)
	addFloat64(c.c.Summary.SampleSum, n)
}

func (c prometheusSummary) UpdateDuration(n time.Time) {
	x := time.Since(n).Seconds()
	atomic.AddUint64(c.c.Summary.SampleCount, 1)
	addFloat64(c.c.Summary.SampleSum, x)
}
