package prometheus

import (
	"fmt"
	"hash/fnv"
	"io"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.unistack.org/micro/v3/meter"
)

var _ meter.Meter = &prometheusMeter{}

type prometheusMeter struct {
	opts         meter.Options
	set          prometheus.Registerer
	counter      map[string]*counters
	floatCounter map[string]*floatCounters
	gauge        map[string]*gauges
	histogram    map[string]*histograms
	summary      map[string]*summaries
	sync.Mutex
}

type counters struct {
	cs map[uint64]*prometheusCounter
}

type gauges struct {
	cs map[uint64]*prometheusGauge
}

type histograms struct {
	cs map[uint64]*prometheusHistogram
}

type summaries struct {
	cs map[uint64]*prometheusSummary
}

type floatCounters struct {
	cs map[uint64]*prometheusFloatCounter
}

func newFloat64(v float64) *float64 {
	nv := v
	return &nv
}

func newString(v string) *string {
	nv := v
	return &nv
}

func NewMeter(opts ...meter.Option) *prometheusMeter {
	return &prometheusMeter{
		set:          prometheus.NewRegistry(), // prometheus.DefaultRegisterer,
		opts:         meter.NewOptions(opts...),
		counter:      make(map[string]*counters),
		floatCounter: make(map[string]*floatCounters),
		gauge:        make(map[string]*gauges),
		histogram:    make(map[string]*histograms),
		summary:      make(map[string]*summaries),
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

	for idx := 0; idx < nl; idx += 2 {
		nlabels[idx] = m.opts.LabelPrefix + nlabels[idx]
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
	nl := len(labels)
	if nl == 0 {
		return nil
	}

	nlabels := make([]string, 0, nl)

	for idx := 0; idx < nl; idx += 2 {
		nlabels = append(nlabels, m.opts.LabelPrefix+labels[idx])
		nlabels = append(nlabels, labels[idx+1])
	}
	return nlabels
}

func (m *prometheusMeter) Name() string {
	return m.opts.Name
}

func (m *prometheusMeter) Counter(name string, labels ...string) meter.Counter {
	m.Lock()
	defer m.Unlock()
	nm := m.buildName(name)
	labels = m.buildLabels(append(m.opts.Labels, labels...)...)
	cd, ok := m.counter[nm]
	h := newHash(labels)
	if !ok {
		cd = &counters{cs: make(map[uint64]*prometheusCounter)}
		c := &prometheusCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.counter[nm] = cd
		return c
	}
	c, ok := cd.cs[h]
	if !ok {
		c = &prometheusCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.counter[nm] = cd
	}
	return c
}

func (m *prometheusMeter) FloatCounter(name string, labels ...string) meter.FloatCounter {
	m.Lock()
	defer m.Unlock()
	nm := m.buildName(name)
	labels = m.buildLabels(append(m.opts.Labels, labels...)...)
	cd, ok := m.floatCounter[nm]
	h := newHash(labels)
	if !ok {
		cd = &floatCounters{cs: make(map[uint64]*prometheusFloatCounter)}
		c := &prometheusFloatCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.floatCounter[nm] = cd
		return c
	}
	c, ok := cd.cs[h]
	if !ok {
		c = &prometheusFloatCounter{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.floatCounter[nm] = cd
	}
	return c
}

func (m *prometheusMeter) Gauge(name string, fn func() float64, labels ...string) meter.Gauge {
	m.Lock()
	defer m.Unlock()
	nm := m.buildName(name)
	labels = m.buildLabels(append(m.opts.Labels, labels...)...)
	cd, ok := m.gauge[nm]
	h := newHash(labels)
	if !ok {
		cd = &gauges{cs: make(map[uint64]*prometheusGauge)}
		c := &prometheusGauge{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.gauge[nm] = cd
		return c
	}
	c, ok := cd.cs[h]
	if !ok {
		c = &prometheusGauge{c: prometheus.NewGauge(prometheus.GaugeOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.gauge[nm] = cd
	}
	return c
}

func (m *prometheusMeter) Histogram(name string, labels ...string) meter.Histogram {
	m.Lock()
	defer m.Unlock()
	nm := m.buildName(name)
	labels = m.buildLabels(append(m.opts.Labels, labels...)...)
	cd, ok := m.histogram[nm]
	h := newHash(labels)
	if !ok {
		cd = &histograms{cs: make(map[uint64]*prometheusHistogram)}
		c := &prometheusHistogram{c: prometheus.NewHistogram(prometheus.HistogramOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.histogram[nm] = cd
		return c
	}
	c, ok := cd.cs[h]
	if !ok {
		c = &prometheusHistogram{c: prometheus.NewHistogram(prometheus.HistogramOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.histogram[nm] = cd
	}
	return c
}

func (m *prometheusMeter) Summary(name string, labels ...string) meter.Summary {
	m.Lock()
	defer m.Unlock()
	nm := m.buildName(name)
	labels = m.buildLabels(append(m.opts.Labels, labels...)...)
	cd, ok := m.summary[nm]
	h := newHash(labels)
	if !ok {
		cd = &summaries{cs: make(map[uint64]*prometheusSummary)}
		c := &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.summary[nm] = cd
		return c
	}
	c, ok := cd.cs[h]
	if !ok {
		c = &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{Name: nm}), labels: labels}
		cd.cs[h] = c
		m.summary[nm] = cd
	}
	return c
}

func (m *prometheusMeter) SummaryExt(name string, window time.Duration, quantiles []float64, labels ...string) meter.Summary {
	m.Lock()
	defer m.Unlock()
	nm := m.buildName(name)
	labels = m.buildLabels(append(m.opts.Labels, labels...)...)
	cd, ok := m.summary[nm]
	h := newHash(labels)
	if !ok {
		cd = &summaries{cs: make(map[uint64]*prometheusSummary)}
		c := &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{
			Name:       nm,
			MaxAge:     window,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}), labels: labels}
		cd.cs[h] = c
		m.summary[nm] = cd
		return c
	}
	c, ok := cd.cs[h]
	if !ok {
		c = &prometheusSummary{c: prometheus.NewSummary(prometheus.SummaryOpts{
			Name:       nm,
			MaxAge:     window,
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}), labels: labels}
		cd.cs[h] = c
		m.summary[nm] = cd
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

	enc := expfmt.NewEncoder(w, expfmt.NewFormat(expfmt.TypeTextPlain))

	for name, metrics := range m.counter {
		mf := &dto.MetricFamily{
			Name:   newString(name),
			Type:   dto.MetricType_GAUGE.Enum(),
			Metric: make([]*dto.Metric, 0, len(metrics.cs)),
		}
		for _, c := range metrics.cs {
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
		}
		mfs = append(mfs, mf)
	}

	for name, metrics := range m.gauge {
		mf := &dto.MetricFamily{
			Name:   newString(name),
			Type:   dto.MetricType_GAUGE.Enum(),
			Metric: make([]*dto.Metric, 0, len(metrics.cs)),
		}
		for _, c := range metrics.cs {
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
		}
		mfs = append(mfs, mf)
	}

	for name, metrics := range m.floatCounter {
		mf := &dto.MetricFamily{
			Name:   newString(name),
			Type:   dto.MetricType_GAUGE.Enum(),
			Metric: make([]*dto.Metric, 0, len(metrics.cs)),
		}
		for _, c := range metrics.cs {
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
		}
		mfs = append(mfs, mf)
	}

	for name, metrics := range m.histogram {
		mf := &dto.MetricFamily{
			Name:   newString(name),
			Type:   dto.MetricType_HISTOGRAM.Enum(),
			Metric: make([]*dto.Metric, 0, len(metrics.cs)),
		}
		for _, c := range metrics.cs {
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
		}
		mfs = append(mfs, mf)
	}

	for name, metrics := range m.summary {
		mf := &dto.MetricFamily{
			Name:   newString(name),
			Type:   dto.MetricType_SUMMARY.Enum(),
			Metric: make([]*dto.Metric, 0, len(metrics.cs)),
		}
		for _, c := range metrics.cs {
			m := &dto.Metric{}
			_ = c.c.Write(m)
			fillMetric(m, c.labels)
			mf.Metric = append(mf.Metric, m)
		}
		mfs = append(mfs, mf)
	}

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
			Name:  newString(labels[idx]),
			Value: newString(labels[idx+1]),
		})
		seen[labels[idx]] = true
	}
	return m
}
