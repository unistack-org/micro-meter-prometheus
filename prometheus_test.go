package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.unistack.org/micro/v3/client"
	"go.unistack.org/micro/v3/codec"
	"go.unistack.org/micro/v3/meter"
)

func TestHash(t *testing.T) {
	m := NewMeter() // meter.Labels("test_key", "test_val"))

	buf := bytes.NewBuffer(nil)

	for i := 0; i < 100000; i++ {
		go func() {
			m.Counter("micro_server_request_total", "code", "16",
				"endpoint", "/clientprofile.ClientProfileService/GetClientProfile",
				"status", "failure").Inc()
			m.Counter("micro_server_request_total", "code", "16",
				"endpoint", "/clientproduct.ClientProductService/GetDepositProducts",
				"status", "failure").Inc()
			m.Counter("micro_server_request_total", "code", "16",
				"endpoint", "/operationsinfo.OperationsInfoService/GetOperations",
				"status", "failure").Inc()
		}()
	}
	_ = m.Write(buf)
	t.Logf("h1: %s\n", buf.Bytes())
}

func TestHistogram(t *testing.T) {
	m := NewMeter()
	name := "test"
	m.Histogram(name, "endpoint").Update(1)
	m.Histogram(name, "endpoint").Update(1)
	m.Histogram(name, "endpoint").Update(5)
	m.Histogram(name, "endpoint").Update(10)
	m.Histogram(name, "endpoint").Update(10)
	m.Histogram(name, "endpoint").Update(30)
	mbuf := bytes.NewBuffer(nil)
	_ = m.Write(mbuf, meter.WriteProcessMetrics(false), meter.WriteFDMetrics(false))

	/*
		if !bytes.Contains(buf.Bytes(), []byte(`micro_server_sum{endpoint="ep1",path="/path1"} 20`)) {
			t.Fatalf("invalid metrics output: %s", buf.Bytes())
		}

		if !bytes.Contains(buf.Bytes(), []byte(`micro_server_count{endpoint="ep1",path="/path1"} 2`)) {
			t.Fatalf("invalid metrics output: %s", buf.Bytes())
		}
	*/
	p := prometheus.NewHistogram(prometheus.HistogramOpts{Name: name})
	p.Observe(1)
	p.Observe(1)
	p.Observe(5)
	p.Observe(10)
	p.Observe(10)
	p.Observe(30)
	mdto := &dto.Metric{}
	_ = p.Write(mdto)
	pbuf := bytes.NewBuffer(nil)
	enc := expfmt.NewEncoder(pbuf, expfmt.NewFormat(expfmt.TypeTextPlain))
	mf := &dto.MetricFamily{Name: &name, Type: dto.MetricType_HISTOGRAM.Enum(), Metric: []*dto.Metric{mdto}}
	_ = enc.Encode(mf)

	if !bytes.Equal(mbuf.Bytes(), pbuf.Bytes()) {
		fmt.Printf("m\n%s\n", mbuf.Bytes())
		fmt.Printf("m\n%s\n", pbuf.Bytes())
	}
}

func TestSummary(t *testing.T) {
	t.Skip()
	name := "micro_server"
	m := NewMeter()
	m.Summary("micro_server").Update(1)
	m.Summary("micro_server").Update(1)
	m.Summary("micro_server").Update(5)
	m.Summary("micro_server").Update(10)
	m.Summary("micro_server").Update(10)
	m.Summary("micro_server").Update(30)
	mbuf := bytes.NewBuffer(nil)
	_ = m.Write(mbuf, meter.WriteProcessMetrics(false), meter.WriteFDMetrics(false))

	if !bytes.Contains(mbuf.Bytes(), []byte(`micro_server_sum 57`)) {
		t.Fatalf("invalid metrics output: %s", mbuf.Bytes())
	}

	if !bytes.Contains(mbuf.Bytes(), []byte(`micro_server_count 6`)) {
		t.Fatalf("invalid metrics output: %s", mbuf.Bytes())
	}

	objectives := make(map[float64]float64)
	for _, c := range meter.DefaultSummaryQuantiles {
		objectives[c] = c
	}
	p := prometheus.NewSummary(prometheus.SummaryOpts{Name: name, Objectives: objectives, MaxAge: meter.DefaultSummaryWindow})
	p.Observe(1)
	p.Observe(1)
	p.Observe(5)
	p.Observe(10)
	p.Observe(10)
	p.Observe(30)
	mdto := &dto.Metric{}
	_ = p.Write(mdto)
	pbuf := bytes.NewBuffer(nil)
	enc := expfmt.NewEncoder(pbuf, expfmt.NewFormat(expfmt.TypeTextPlain))
	mf := &dto.MetricFamily{Name: &name, Type: dto.MetricType_SUMMARY.Enum(), Metric: []*dto.Metric{mdto}}
	_ = enc.Encode(mf)

	if !bytes.Equal(mbuf.Bytes(), pbuf.Bytes()) {
		fmt.Printf("m\n%s\n", mbuf.Bytes())
		fmt.Printf("m\n%s\n", pbuf.Bytes())
	}
}

func TestStd(t *testing.T) {
	m := NewMeter(meter.WriteProcessMetrics(true), meter.WriteFDMetrics(true))
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	buf := bytes.NewBuffer(nil)
	_ = m.Write(buf)
	if !bytes.Contains(buf.Bytes(), []byte(`go_goroutine`)) {
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
}

func TestWrapper(t *testing.T) {
	m := NewMeter() // meter.Labels("test_key", "test_val"))

	ctx := context.Background()

	c := client.NewClient(client.Meter(m))
	if err := c.Init(); err != nil {
		t.Fatal(err)
	}
	rsp := &codec.Frame{}
	req := &codec.Frame{}
	err := c.Call(ctx, c.NewRequest("svc2", "Service.Method", req), rsp)
	_, _ = rsp, err
	buf := bytes.NewBuffer(nil)
	_ = m.Write(buf, meter.WriteProcessMetrics(false), meter.WriteFDMetrics(false))
	if !bytes.Contains(buf.Bytes(), []byte(`micro_client_request_inflight{endpoint="Service.Method"} 0`)) {
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
}

func TestMultiple(t *testing.T) {
	m := NewMeter() // meter.Labels("test_key", "test_val"))

	m.Counter("micro_server", "endpoint", "ep1", "path", "/path1").Inc()
	m.Counter("micro_server", "endpoint", "ep1", "path", "/path1").Inc()

	m.Counter("micro_server", "endpoint", "ep2", "path", "/path2").Inc()
	m.Counter("micro_server", "endpoint", "ep2", "path", "/path2").Inc()

	m.Counter("micro_server", "endpoint", "ep3", "path", "/path3", "status", "success").Inc()

	buf := bytes.NewBuffer(nil)
	_ = m.Write(buf, meter.WriteProcessMetrics(false), meter.WriteFDMetrics(false))
	if !bytes.Contains(buf.Bytes(), []byte(`micro_server{endpoint="ep1",path="/path1"} 2`)) {
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
	if !bytes.Contains(buf.Bytes(), []byte(`micro_server{endpoint="ep3",path="/path3",status="success"} 1`)) {
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
}
