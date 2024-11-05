package prometheus

import (
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.unistack.org/micro/v3/meter"
	xpool "go.unistack.org/micro/v3/util/xpool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ meter.Meter = (*prometheusMeter)(nil)

type prometheusMeter struct {
	opts         meter.Options
	set          prometheus.Registerer
	counter      map[uint64]*prometheusCounter
	floatCounter map[uint64]*prometheusFloatCounter
	gauge        map[uint64]*prometheusGauge
	histogram    map[uint64]*prometheusHistogram
	summary      map[uint64]*prometheusSummary
	mfPool       xpool.Pool[*dto.MetricFamily]
	mu           sync.Mutex
}

func NewMeter(opts ...meter.Option) *prometheusMeter {
	return &prometheusMeter{
		set:          prometheus.NewRegistry(), // prometheus.DefaultRegisterer,
		opts:         meter.NewOptions(opts...),
		counter:      make(map[uint64]*prometheusCounter),
		floatCounter: make(map[uint64]*prometheusFloatCounter),
		gauge:        make(map[uint64]*prometheusGauge),
		histogram:    make(map[uint64]*prometheusHistogram),
		summary:      make(map[uint64]*prometheusSummary),
		mfPool: xpool.NewPool[*dto.MetricFamily](func() *dto.MetricFamily {
			return &dto.MetricFamily{}
		}),
	}
}

func (m *prometheusMeter) Name() string {
	return m.opts.Name
}

func (m *prometheusMeter) Counter(name string, labels ...string) meter.Counter {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	m.mu.Lock()
	mc, ok := m.counter[h]
	m.mu.Unlock()
	if !ok {
		var v float64
		mc = &prometheusCounter{
			name: name,
			c: &dto.Metric{
				Gauge: &dto.Gauge{Value: &v},
				Label: labelMetric(clabels),
			},
		}
		m.mu.Lock()
		m.counter[h] = mc
		m.mu.Unlock()
	}
	return mc
}

func (m *prometheusMeter) FloatCounter(name string, labels ...string) meter.FloatCounter {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	m.mu.Lock()
	mc, ok := m.floatCounter[h]
	m.mu.Unlock()
	if !ok {
		var v float64
		mc = &prometheusFloatCounter{
			name: name,
			c: &dto.Metric{
				Gauge: &dto.Gauge{Value: &v},
				Label: labelMetric(clabels),
			},
		}
		m.mu.Lock()
		m.floatCounter[h] = mc
		m.mu.Unlock()
	}
	return mc
}

func (m *prometheusMeter) Gauge(name string, fn func() float64, labels ...string) meter.Gauge {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	m.mu.Lock()
	mc, ok := m.gauge[h]
	m.mu.Unlock()
	if !ok {
		var v float64
		mc = &prometheusGauge{
			name: name,
			c: &dto.Metric{
				Gauge: &dto.Gauge{Value: &v},
				Label: labelMetric(clabels),
			},
		}
		m.mu.Lock()
		m.gauge[h] = mc
		m.mu.Unlock()
	}
	return mc
}

func (m *prometheusMeter) Histogram(name string, labels ...string) meter.Histogram {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	m.mu.Lock()
	mc, ok := m.histogram[h]
	m.mu.Unlock()
	if !ok {
		var c uint64
		var s float64
		buckets := make([]float64, len(prometheus.DefBuckets))
		copy(buckets, prometheus.DefBuckets)
		mdto := &dto.Metric{
			Histogram: &dto.Histogram{
				SampleCount:      &c,
				SampleSum:        &s,
				CreatedTimestamp: timestamppb.Now(),
				Bucket:           make([]*dto.Bucket, len(buckets)),
			},
			Label: labelMetric(clabels),
		}
		for idx, b := range buckets {
			var cc uint64
			mdto.Histogram.Bucket[idx] = &dto.Bucket{CumulativeCount: &cc, UpperBound: &b}
		}
		mc = &prometheusHistogram{
			name: name,
			c:    mdto,
		}
		m.mu.Lock()
		m.histogram[h] = mc
		m.mu.Unlock()
	}
	return mc
}

func (m *prometheusMeter) Summary(name string, labels ...string) meter.Summary {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	m.mu.Lock()
	mc, ok := m.summary[h]
	m.mu.Unlock()
	if !ok {
		var c uint64
		var s float64
		mc = &prometheusSummary{
			name: name,
			c: &dto.Metric{
				Summary: &dto.Summary{
					SampleCount:      &c,
					SampleSum:        &s,
					CreatedTimestamp: timestamppb.Now(),
				},
				Label: labelMetric(clabels),
			},
		}
		m.mu.Lock()
		m.summary[h] = mc
		m.mu.Unlock()
	}
	return mc
}

func (m *prometheusMeter) SummaryExt(name string, window time.Duration, quantiles []float64, labels ...string) meter.Summary {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	m.mu.Lock()
	mc, ok := m.summary[h]
	m.mu.Lock()
	if !ok {
		var c uint64
		var s float64
		mc = &prometheusSummary{
			name: name,
			c: &dto.Metric{
				Summary: &dto.Summary{
					SampleCount: &c,
					SampleSum:   &s,
				},
				Label: labelMetric(clabels),
			},
		}
		m.mu.Lock()
		m.summary[h] = mc
		m.mu.Unlock()
	}
	return mc
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

	m.mu.Lock()

	for _, c := range m.counter {
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_GAUGE.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
	}

	for _, c := range m.floatCounter {
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_GAUGE.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
	}

	for _, c := range m.gauge {
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_GAUGE.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
	}

	for _, c := range m.histogram {
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_HISTOGRAM.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
	}

	for _, c := range m.summary {
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_SUMMARY.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
	}

	m.mu.Unlock()

	for _, mf := range mfs {
		_ = enc.Encode(mf)
		mf.Reset()
		m.mfPool.Put(mf)
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

func labelMetric(labels []string) []*dto.LabelPair {
	dtoLabels := make([]*dto.LabelPair, 0, len(labels)/2)
	for idx := 0; idx < len(labels); idx += 2 {
		dtoLabels = append(dtoLabels, &dto.LabelPair{
			Name:  &(labels[idx]),
			Value: &(labels[idx+1]),
		})
	}
	return dtoLabels
}

func newHash(n string, l []string) uint64 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(n); i++ {
		h ^= uint64(n[i])
		h *= 1099511628211
	}
	for _, s := range l {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
	}
	return h
}
