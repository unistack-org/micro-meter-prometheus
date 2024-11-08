package prometheus

import (
	"sync/atomic"
	"time"

	dto "github.com/prometheus/client_model/go"
)

type prometheusSummary struct {
	name        string
	c           *dto.Metric
	sampleCount uint64
	SampleSum   float64
}

func (c *prometheusSummary) Update(n float64) {
	atomic.AddUint64(&(c.sampleCount), 1)
	addFloat64(&(c.SampleSum), n)
}

func (c *prometheusSummary) UpdateDuration(t time.Time) {
	n := time.Since(t).Seconds()
	atomic.AddUint64(&(c.sampleCount), 1)
	addFloat64(&(c.SampleSum), n)
}
