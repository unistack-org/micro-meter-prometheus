package prometheus

import (
	"math"
	"sync/atomic"
	"unsafe"

	dto "github.com/prometheus/client_model/go"
)

type prometheusCounter struct {
	name string
	c    *dto.Metric
}

func (c *prometheusCounter) Add(n int) {
	addFloat64(c.c.Gauge.Value, float64(n))
}

func (c *prometheusCounter) Dec() {
	addFloat64(c.c.Gauge.Value, float64(-1))
}

func (c *prometheusCounter) Inc() {
	addFloat64(c.c.Gauge.Value, float64(1))
}

func (c *prometheusCounter) Get() uint64 {
	return uint64(getFloat64(c.c.Gauge.Value))
}

func (c *prometheusCounter) Set(n uint64) {
	setFloat64(c.c.Gauge.Value, math.Float64frombits(n))
}

type prometheusFloatCounter struct {
	name string
	c    *dto.Metric
}

func (c *prometheusFloatCounter) Add(n float64) {
	addFloat64(c.c.Gauge.Value, n)
}

func (c *prometheusFloatCounter) Dec() {
	addFloat64(c.c.Gauge.Value, float64(-1))
}

func (c *prometheusFloatCounter) Inc() {
	addFloat64(c.c.Gauge.Value, float64(1))
}

func (c *prometheusFloatCounter) Get() float64 {
	return getFloat64(c.c.Gauge.Value)
}

func (c *prometheusFloatCounter) Set(n float64) {
	setFloat64(c.c.Gauge.Value, n)
}

func (c *prometheusFloatCounter) Sub(n float64) {
	addFloat64(c.c.Gauge.Value, -n)
}

func setFloat64(_addr *float64, value float64) float64 {
	addr := (*uint64)(unsafe.Pointer(_addr))
	for {
		x := atomic.LoadUint64(addr)
		if atomic.CompareAndSwapUint64(addr, x, math.Float64bits(value)) {
			return value
		}
	}
}

func addFloat64(_addr *float64, delta float64) float64 {
	addr := (*uint64)(unsafe.Pointer(_addr))
	for {
		x := atomic.LoadUint64(addr)
		y := math.Float64frombits(x) + delta
		if atomic.CompareAndSwapUint64(addr, x, math.Float64bits(y)) {
			return y
		}
	}
}

func getFloat64(_addr *float64) float64 {
	addr := (*uint64)(unsafe.Pointer(_addr))
	x := atomic.LoadUint64(addr)
	y := math.Float64frombits(x)
	return y
}
