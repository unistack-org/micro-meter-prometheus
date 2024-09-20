package prometheus

import (
	"fmt"
	"hash/fnv"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.unistack.org/micro/v3/meter"
)

var _ meter.Meter = (*prometheusMeter)(nil)

type prometheusMeter struct {
	opts         meter.Options
	set          prometheus.Registerer
	counter      *sync.Map
	floatCounter *sync.Map
	gauge        *sync.Map
	histogram    *sync.Map
	summary      *sync.Map
	sync.Mutex
}

type counters struct {
	cs *sync.Map
}

type gauges struct {
	cs *sync.Map
}

type histograms struct {
	cs *sync.Map
}

type summaries struct {
	cs *sync.Map
}

type floatCounters struct {
	cs *sync.Map
}

func NewMeter(opts ...meter.Option) *prometheusMeter {
	return &prometheusMeter{
		set:          prometheus.NewRegistry(), // prometheus.DefaultRegisterer,
		opts:         meter.NewOptions(opts...),
		counter:      &sync.Map{},
		floatCounter: &sync.Map{},
		gauge:        &sync.Map{},
		histogram:    &sync.Map{},
		summary:      &sync.Map{},
	}
}

func (m *prometheusMeter) Name() string {
	return m.opts.Name
}

func (m *prometheusMeter) Counter(name string, labels ...string) meter.Counter {
	m.Lock()
	defer m.Unlock()
	labels = append(m.opts.Labels, labels...)
	vcd, ok := m.counter.Load(name)
	h := newHash(labels)
	if !ok {
		cd := &counters{cs: &sync.Map{}}
		c := &prometheusCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.counter.Store(name, cd)
		return c
	}
	cd := vcd.(*counters)
	vc, ok := cd.cs.Load(h)
	if !ok {
		c := &prometheusCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.counter.Store(name, cd)
		return c
	}
	c := vc.(*prometheusCounter)
	return c
}

func (m *prometheusMeter) FloatCounter(name string, labels ...string) meter.FloatCounter {
	m.Lock()
	defer m.Unlock()
	labels = append(m.opts.Labels, labels...)
	vcd, ok := m.floatCounter.Load(name)
	h := newHash(labels)
	if !ok {
		cd := &floatCounters{cs: &sync.Map{}}
		c := &prometheusFloatCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.floatCounter.Store(name, cd)
		return c
	}
	cd := vcd.(*floatCounters)
	vc, ok := cd.cs.Load(h)
	if !ok {
		c := &prometheusFloatCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.floatCounter.Store(name, cd)
		return c
	}
	c := vc.(*prometheusFloatCounter)
	return c
}

func (m *prometheusMeter) Gauge(name string, fn func() float64, labels ...string) meter.Gauge {
	m.Lock()
	defer m.Unlock()
	labels = append(m.opts.Labels, labels...)
	vcd, ok := m.gauge.Load(name)
	h := newHash(labels)
	if !ok {
		cd := &gauges{cs: &sync.Map{}}
		c := &prometheusGauge{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.gauge.Store(name, cd)
		return c
	}
	cd := vcd.(*gauges)
	vc, ok := cd.cs.Load(h)
	if !ok {
		c := &prometheusGauge{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.gauge.Store(name, cd)
		return c
	}
	c := vc.(*prometheusGauge)
	return c
}

func (m *prometheusMeter) Histogram(name string, labels ...string) meter.Histogram {
	m.Lock()
	defer m.Unlock()
	labels = append(m.opts.Labels, labels...)
	vcd, ok := m.histogram.Load(name)
	h := newHash(labels)
	if !ok {
		cd := &histograms{cs: &sync.Map{}}
		c := &prometheusHistogram{c: prometheus.NewHistogram(prometheus.HistogramOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.histogram.Store(name, cd)
		return c
	}
	cd := vcd.(*histograms)
	vc, ok := cd.cs.Load(h)
	if !ok {
		c := &prometheusHistogram{c: prometheus.NewHistogram(prometheus.HistogramOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.histogram.Store(name, cd)
		return c
	}
	c := vc.(*prometheusHistogram)
	return c
}

func (m *prometheusMeter) Summary(name string, labels ...string) meter.Summary {
	m.Lock()
	defer m.Unlock()
	labels = append(m.opts.Labels, labels...)
	vcd, ok := m.summary.Load(name)
	h := newHash(labels)
	if !ok {
		cd := &summaries{cs: &sync.Map{}}
		c := &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.summary.Store(name, cd)
		return c
	}
	cd := vcd.(*summaries)
	vc, ok := cd.cs.Load(h)
	if !ok {
		c := &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{Name: name}), labels: labels}
		cd.cs.Store(h, c)
		m.summary.Store(name, cd)
		return c
	}
	c := vc.(*prometheusSummary)
	return c
}

func (m *prometheusMeter) SummaryExt(name string, window time.Duration, quantiles []float64, labels ...string) meter.Summary {
	m.Lock()
	defer m.Unlock()
	labels = append(m.opts.Labels, labels...)
	vcd, ok := m.summary.Load(name)
	h := newHash(labels)
	if !ok {
		cd := &summaries{cs: &sync.Map{}}
		c := &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{
			Name:       name,
			MaxAge:     window,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}), labels: labels}
		cd.cs.Store(h, c)
		m.summary.Store(name, cd)
		return c
	}
	cd := vcd.(*summaries)
	vc, ok := cd.cs.Load(h)
	if !ok {
		c := &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{
			Name:       name,
			MaxAge:     window,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}), labels: labels}
		cd.cs.Store(h, c)
		m.summary.Store(name, cd)
		return c
	}
	c := vc.(*prometheusSummary)
	return c
}

func (m *prometheusMeter) Init(opts ...meter.Option) error {
	for _, o := range opts {
		o(&m.opts)
	}

	if m.opts.WriteProcessMetrics || m.opts.WriteFDMetrics {
		pc := collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})
		_ = m.set.Register(pc)
		gc := collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")}))
		_ = m.set.Register(gc)
	}

	return nil
}

func (m *prometheusMeter) Write(w io.Writer, opts ...meter.Option) error {
	options := m.opts
	for _, o := range opts {
		o(&options)
	}

	g, ok := m.set.(prometheus.Gatherer)
	if !ok {
		return fmt.Errorf("set type %T not prometheus.Gatherer", m.set)
	}

	mfs, err := g.Gather()
	if err != nil {
		return err
	}

	enc := expfmt.NewEncoder(w, expfmt.NewFormat(expfmt.TypeTextPlain))

	m.counter.Range(func(k, v any) bool {
		name := k.(string)
		mf := &dto.MetricFamily{
			Name: &name,
			Type: dto.MetricType_GAUGE.Enum(),
		}
		v.(*counters).cs.Range(func(_, nv any) bool {
			c := nv.(*prometheusCounter)
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
			return true
		})
		mfs = append(mfs, mf)
		return true
	})

	m.gauge.Range(func(k, v any) bool {
		name := k.(string)
		mf := &dto.MetricFamily{
			Name: &name,
			Type: dto.MetricType_GAUGE.Enum(),
		}
		v.(*gauges).cs.Range(func(_, nv any) bool {
			c := nv.(*prometheusGauge)
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
			return true
		})
		mfs = append(mfs, mf)
		return true
	})

	m.floatCounter.Range(func(k, v any) bool {
		name := k.(string)
		mf := &dto.MetricFamily{
			Name: &name,
			Type: dto.MetricType_GAUGE.Enum(),
		}
		v.(*floatCounters).cs.Range(func(_, nv any) bool {
			c := nv.(*prometheusFloatCounter)
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
			return true
		})
		mfs = append(mfs, mf)
		return true
	})

	m.histogram.Range(func(k, v any) bool {
		name := k.(string)
		mf := &dto.MetricFamily{
			Name: &name,
			Type: dto.MetricType_HISTOGRAM.Enum(),
		}
		v.(*histograms).cs.Range(func(_, nv any) bool {
			c := nv.(*prometheusHistogram)
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
			return true
		})
		mfs = append(mfs, mf)
		return true
	})

	m.summary.Range(func(k, v any) bool {
		name := k.(string)
		mf := &dto.MetricFamily{
			Name: &name,
			Type: dto.MetricType_SUMMARY.Enum(),
		}
		v.(*summaries).cs.Range(func(_, nv any) bool {
			c := nv.(*prometheusSummary)
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
			return true
		})
		mfs = append(mfs, mf)
		return true
	})

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
		set:          m.set,
		opts:         options,
		floatCounter: m.floatCounter,
		counter:      m.counter,
		gauge:        m.gauge,
		histogram:    m.histogram,
		summary:      m.summary,
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
	c      prometheus.Gauge
	labels []string
}

func (c *prometheusCounter) Add(n int) {
	c.c.Add(float64(n))
}

func (c *prometheusCounter) Dec() {
	c.c.Dec()
}

func (c *prometheusCounter) Inc() {
	c.c.Inc()
}

func (c *prometheusCounter) Get() uint64 {
	m := &dto.Metric{}
	if err := c.c.Write(m); err != nil {
		return 0
	}
	return uint64(m.GetGauge().GetValue())
}

func (c *prometheusCounter) Set(n uint64) {
	c.c.Set(float64(n))
}

type prometheusFloatCounter struct {
	c      prometheus.Gauge
	labels []string
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
	c      prometheus.Gauge
	labels []string
}

func (c prometheusGauge) Get() float64 {
	m := &dto.Metric{}
	if err := c.c.Write(m); err != nil {
		return 0
	}
	return float64(m.GetGauge().GetValue())
}

type prometheusHistogram struct {
	c      prometheus.Histogram
	labels []string
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
	c      prometheus.Summary
	labels []string
}

func (c prometheusSummary) Update(n float64) {
	c.c.Observe(n)
}

func (c prometheusSummary) UpdateDuration(n time.Time) {
	c.c.Observe(time.Since(n).Seconds())
}

func newHash(labels []string) uint64 {
	labels = meter.BuildLabels(labels...)
	h := fnv.New64a()
	for _, l := range labels {
		h.Write([]byte(l))
	}
	return h.Sum64()
}

func fillMetric(m *dto.Metric, labels []string) *dto.Metric {
	var ok bool
	seen := make(map[string]bool, len(labels)/2)
	m.Label = make([]*dto.LabelPair, 0, len(labels)/2)
	for idx := 0; idx < len(labels); idx += 2 {
		if _, ok = seen[labels[idx]]; ok {
			continue
		}
		m.Label = append(m.Label, &dto.LabelPair{
			Name:  &(labels[idx]),
			Value: &(labels[idx+1]),
		})
		seen[labels[idx]] = true
	}
	return m
}
