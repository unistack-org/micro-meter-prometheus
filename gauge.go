package prometheus

import dto "github.com/prometheus/client_model/go"

type prometheusGauge struct {
	name string
	c    *dto.Metric
	n    float64
}

func (c *prometheusGauge) Get() float64 {
	return getFloat64(&(c.n))
}
