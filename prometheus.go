package prometheus

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.unistack.org/micro/v3/meter"
)

type prometheusMeter struct {
	opts         meter.Options
	set          prometheus.Registerer
	counter      map[string]prometheusCounter
	floatCounter map[string]prometheusFloatCounter
	gauge        map[string]prometheusGauge
	histogram    map[string]prometheusHistogram
	summary      map[string]prometheusSummary
	sync.Mutex
}

func NewMeter(opts ...meter.Option) meter.Meter {
	return &prometheusMeter{
		set:          prometheus.DefaultRegisterer,
		opts:         meter.NewOptions(opts...),
		counter:      make(map[string]prometheusCounter),
		floatCounter: make(map[string]prometheusFloatCounter),
		gauge:        make(map[string]prometheusGauge),
		histogram:    make(map[string]prometheusHistogram),
		summary:      make(map[string]prometheusSummary),
	}
}

func (m *prometheusMeter) buildMetric(name string, labels ...string) string {
	if len(m.opts.MetricPrefix) > 0 {
		name = m.opts.MetricPrefix + name
	}

	nl := len(m.opts.Labels) + len(labels)
	if nl == 0 {
		return name
	}

	nlabels := make([]string, 0, nl)
	nlabels = append(nlabels, m.opts.Labels...)
	nlabels = append(nlabels, labels...)

	if len(m.opts.LabelPrefix) == 0 {
		return meter.BuildName(name, nlabels...)
	}

	for idx := 0; idx < nl; idx++ {
		nlabels[idx] = m.opts.LabelPrefix + nlabels[idx]
		idx++
	}
	return meter.BuildName(name, nlabels...)
}

func (m *prometheusMeter) buildName(name string) string {
	if len(m.opts.MetricPrefix) > 0 {
		name = m.opts.MetricPrefix + name
	}
	return name
}

func (m *prometheusMeter) buildLabels(labels ...string) []string {
	nl := len(m.opts.Labels) + len(labels)
	if nl == 0 {
		return nil
	}

	nlabels := make([]string, 0, nl)
	nlabels = append(nlabels, m.opts.Labels...)
	nlabels = append(nlabels, labels...)

	for idx := 0; idx < nl; idx++ {
		nlabels[idx] = m.opts.LabelPrefix + nlabels[idx]
		idx++
	}
	return nlabels
}

func (m *prometheusMeter) Name() string {
	return m.opts.Name
}

func (m *prometheusMeter) mapLabels(labels ...string) map[string]string {
	labels = m.buildLabels(labels...)
	elementMap := make(map[string]string, len(labels)/2)
	for idx := 0; idx < len(labels); idx++ {
		elementMap[labels[idx]] = labels[idx+1]
		idx++
	}
	return elementMap
}

func (m *prometheusMeter) Counter(name string, labels ...string) meter.Counter {
	m.Lock()
	defer m.Unlock()

	nm := m.buildName(name)
	c, ok := m.counter[nm]
	if !ok {
		nc := prometheus.NewGauge(prometheus.GaugeOpts{Name: nm, ConstLabels: m.mapLabels(labels...)})
		m.set.MustRegister(nc)
		c = prometheusCounter{c: nc}
		m.counter[nm] = c
	}

	return c
}

func (m *prometheusMeter) FloatCounter(name string, labels ...string) meter.FloatCounter {
	m.Lock()
	defer m.Unlock()

	nm := m.buildName(name)
	c, ok := m.floatCounter[nm]
	if !ok {
		nc := prometheus.NewGauge(prometheus.GaugeOpts{Name: nm, ConstLabels: m.mapLabels(labels...)})
		m.set.MustRegister(nc)
		c = prometheusFloatCounter{c: nc}
		m.floatCounter[nm] = c
	}

	return c
}

func (m *prometheusMeter) Gauge(name string, fn func() float64, labels ...string) meter.Gauge {
	m.Lock()
	defer m.Unlock()

	nm := m.buildName(name)
	c, ok := m.gauge[nm]
	if !ok {
		nc := prometheus.NewGauge(prometheus.GaugeOpts{Name: nm, ConstLabels: m.mapLabels(labels...)})
		m.set.MustRegister(nc)
		c = prometheusGauge{c: nc}
		m.gauge[nm] = c
	}

	return c
}

func (m *prometheusMeter) Histogram(name string, labels ...string) meter.Histogram {
	m.Lock()
	defer m.Unlock()

	nm := m.buildName(name)
	c, ok := m.histogram[nm]
	if !ok {
		nc := prometheus.NewHistogram(prometheus.HistogramOpts{Name: nm, ConstLabels: m.mapLabels(labels...)})
		m.set.MustRegister(nc)
		c = prometheusHistogram{c: nc}
		m.histogram[nm] = c
	}

	return c
}

func (m *prometheusMeter) Summary(name string, labels ...string) meter.Summary {
	m.Lock()
	defer m.Unlock()

	nm := m.buildName(name)
	c, ok := m.summary[nm]
	if !ok {
		nc := prometheus.NewSummary(prometheus.SummaryOpts{Name: nm, ConstLabels: m.mapLabels(labels...)})
		m.set.MustRegister(nc)
		c = prometheusSummary{c: nc}
		m.summary[nm] = c
	}

	return c
}

func (m *prometheusMeter) SummaryExt(name string, window time.Duration, quantiles []float64, labels ...string) meter.Summary {
	m.Lock()
	defer m.Unlock()

	nm := m.buildName(name)
	c, ok := m.summary[nm]
	if !ok {
		nc := prometheus.NewSummary(prometheus.SummaryOpts{
			Name:        nm,
			ConstLabels: m.mapLabels(labels...),
			MaxAge:      window,
			Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		})
		m.set.MustRegister(nc)
		c = prometheusSummary{c: nc}
		m.summary[nm] = c
	}

	return c
}

func (m *prometheusMeter) Init(opts ...meter.Option) error {
	for _, o := range opts {
		o(&m.opts)
	}

	return nil
}

func (m *prometheusMeter) Write(w io.Writer, opts ...meter.Option) error {
	options := m.opts
	for _, o := range opts {
		o(&options)
	}

	if options.WriteProcessMetrics || options.WriteFDMetrics {
		c := collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
		_ = m.set.Register(c)
	}

	g, ok := m.set.(prometheus.Gatherer)
	if !ok {
		return fmt.Errorf("set type %T not prometheus.Gatherer", m.set)
	}

	mfs, err := g.Gather()
	if err != nil {
		return err
	}

	enc := expfmt.NewEncoder(w, expfmt.FmtText)
	for _, mf := range mfs {
		_ = enc.Encode(mf)
	}

	if closer, ok := enc.(io.Closer); ok {
		_ = closer.Close()
	}

	return nil
}

func (m *prometheusMeter) Clone(opts ...meter.Option) meter.Meter {
	options := m.opts
	for _, o := range opts {
		o(&options)
	}

	return &prometheusMeter{
		set:       m.set,
		opts:      options,
		counter:   m.counter,
		gauge:     m.gauge,
		histogram: m.histogram,
		summary:   m.summary,
	}
}

func (m *prometheusMeter) Options() meter.Options {
	return m.opts
}

func (m *prometheusMeter) String() string {
	return "prometheus"
}

func (m *prometheusMeter) Set(opts ...meter.Option) meter.Meter {
	nm := &prometheusMeter{opts: m.opts}
	for _, o := range opts {
		o(&nm.opts)
	}
	nm.set = prometheus.NewRegistry()
	return nm
}

type prometheusCounter struct {
	c prometheus.Gauge
}

func (c prometheusCounter) Add(n int) {
	c.c.Add(float64(n))
}

func (c prometheusCounter) Dec() {
	c.c.Dec()
}

func (c prometheusCounter) Inc() {
	c.c.Inc()
}

func (c prometheusCounter) Get() uint64 {
	m := &dto.Metric{}
	if err := c.c.Write(m); err != nil {
		return 0
	}
	return uint64(m.GetGauge().GetValue())
}

func (c prometheusCounter) Set(n uint64) {
	c.c.Set(float64(n))
}

type prometheusFloatCounter struct {
	c prometheus.Gauge
}

func (c prometheusFloatCounter) Add(n float64) {
	c.c.Add(n)
}

func (c prometheusFloatCounter) Get() float64 {
	m := &dto.Metric{}
	if err := c.c.Write(m); err != nil {
		return 0
	}
	return m.GetGauge().GetValue()
}

func (c prometheusFloatCounter) Set(n float64) {
	c.c.Set(n)
}

func (c prometheusFloatCounter) Sub(n float64) {
	c.c.Add(-n)
}

type prometheusGauge struct {
	c prometheus.Gauge
}

func (c prometheusGauge) Get() float64 {
	m := &dto.Metric{}
	if err := c.c.Write(m); err != nil {
		return 0
	}
	return float64(m.GetGauge().GetValue())
}

type prometheusHistogram struct {
	c prometheus.Histogram
}

func (c prometheusHistogram) Reset() {
}

func (c prometheusHistogram) Update(n float64) {
	c.c.Observe(n)
}

func (c prometheusHistogram) UpdateDuration(n time.Time) {
	c.c.Observe(time.Since(n).Seconds())
}

type prometheusSummary struct {
	c prometheus.Summary
}

func (c prometheusSummary) Update(n float64) {
	c.c.Observe(n)
}

func (c prometheusSummary) UpdateDuration(n time.Time) {
	c.c.Observe(time.Since(n).Seconds())
}
