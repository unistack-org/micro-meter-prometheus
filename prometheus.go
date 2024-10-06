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
	counter      *sync.Map
	floatCounter *sync.Map
	gauge        *sync.Map
	histogram    *sync.Map
	summary      *sync.Map
	mfPool       xpool.Pool[*dto.MetricFamily]
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
	mc, ok := m.counter.Load(h)
	if !ok {
		var v float64
		mc = &prometheusCounter{
			name: name,
			c: &dto.Metric{
				Gauge: &dto.Gauge{Value: &v},
				Label: labelMetric(clabels),
			},
		}
		m.counter.Store(h, mc)
	}
	return mc.(*prometheusCounter)
}

func (m *prometheusMeter) FloatCounter(name string, labels ...string) meter.FloatCounter {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	mc, ok := m.floatCounter.Load(h)
	if !ok {
		var v float64
		mc = &prometheusFloatCounter{
			name: name,
			c: &dto.Metric{
				Gauge: &dto.Gauge{Value: &v},
				Label: labelMetric(clabels),
			},
		}
		m.floatCounter.Store(h, mc)
	}
	return mc.(*prometheusFloatCounter)
}

func (m *prometheusMeter) Gauge(name string, fn func() float64, labels ...string) meter.Gauge {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	mc, ok := m.gauge.Load(h)
	if !ok {
		var v float64
		mc = &prometheusGauge{
			name: name,
			c: &dto.Metric{
				Gauge: &dto.Gauge{Value: &v},
				Label: labelMetric(clabels),
			},
		}
		m.gauge.Store(h, mc)
	}
	return mc.(*prometheusGauge)
}

func (m *prometheusMeter) Histogram(name string, labels ...string) meter.Histogram {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	mc, ok := m.histogram.Load(h)
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

		m.histogram.Store(h, mc)
	}
	return mc.(*prometheusHistogram)
}

func (m *prometheusMeter) Summary(name string, labels ...string) meter.Summary {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	mc, ok := m.summary.Load(h)
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
		m.summary.Store(h, mc)
	}
	return mc.(*prometheusSummary)
}

func (m *prometheusMeter) SummaryExt(name string, window time.Duration, quantiles []float64, labels ...string) meter.Summary {
	clabels := meter.BuildLabels(append(m.opts.Labels, labels...)...)
	h := newHash(name, clabels)
	mc, ok := m.summary.Load(h)
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
		m.summary.Store(h, mc)
	}
	return mc.(*prometheusSummary)
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
		c := v.(*prometheusCounter)
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_GAUGE.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
		return true
	})

	m.floatCounter.Range(func(k, v any) bool {
		c := v.(*prometheusFloatCounter)
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_GAUGE.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
		return true
	})

	m.gauge.Range(func(k, v any) bool {
		c := v.(*prometheusGauge)
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_GAUGE.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
		return true
	})

	m.histogram.Range(func(k, v any) bool {
		c := v.(*prometheusHistogram)
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_HISTOGRAM.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
		return true
	})

	m.summary.Range(func(k, v any) bool {
		c := v.(*prometheusSummary)
		mf := m.mfPool.Get()
		mf.Name = &c.name
		mf.Type = dto.MetricType_SUMMARY.Enum()
		mf.Metric = append(mf.Metric, c.c)
		mfs = append(mfs, mf)
		return true
	})

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
