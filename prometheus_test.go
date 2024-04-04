package prometheus

import (
	"bytes"
	"context"
	"testing"

	"go.unistack.org/micro/v4/client"
	"go.unistack.org/micro/v4/codec"
	"go.unistack.org/micro/v4/meter"
	"go.unistack.org/micro/v4/meter/wrapper"
)

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

func TestBuildName(t *testing.T) {
	m := NewMeter()
	check := `micro_foo{micro_aaa="b",micro_bar="baz",micro_ccc="d"}`
	name := m.buildMetric("foo", "bar", "baz", "aaa", "b", "ccc", "d")
	if name != check {
		t.Fatalf("metric name error: %s != %s", name, check)
	}

	cnt := m.Counter("counter", "key", "val")
	cnt.Inc()
}

func TestWrapper(t *testing.T) {
	m := NewMeter() // meter.Labels("test_key", "test_val"))

	w := wrapper.NewClientWrapper(
		wrapper.ServiceName("svc1"),
		wrapper.ServiceVersion("0.0.1"),
		wrapper.ServiceID("12345"),
		wrapper.Meter(m),
	)
	_ = w
	ctx := context.Background()

	c := client.NewClient()
	if err := c.Init(); err != nil {
		t.Fatal(err)
	}
	rsp := &codec.Frame{}
	req := &codec.Frame{}
	err := c.Call(ctx, c.NewRequest("svc2", "Service.Method", req), rsp)
	_, _ = rsp, err
	buf := bytes.NewBuffer(nil)
	_ = m.Write(buf, meter.WriteProcessMetrics(false), meter.WriteFDMetrics(false))
	if !bytes.Contains(buf.Bytes(), []byte(`micro_client_request_inflight{micro_endpoint="svc2.Service.Method"} 0`)) {
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
}

func TestMultiple(t *testing.T) {
	m := NewMeter() // meter.Labels("test_key", "test_val"))

	m.Counter("server", "endpoint", "ep1", "path", "/path1").Inc()
	m.Counter("server", "endpoint", "ep1", "path", "/path1").Inc()

	m.Counter("server", "endpoint", "ep2", "path", "/path2").Inc()
	m.Counter("server", "endpoint", "ep2", "path", "/path2").Inc()

	m.Counter("server", "endpoint", "ep3", "path", "/path3", "status", "success").Inc()

	buf := bytes.NewBuffer(nil)
	_ = m.Write(buf, meter.WriteProcessMetrics(false), meter.WriteFDMetrics(false))
	if !bytes.Contains(buf.Bytes(), []byte(`micro_server{micro_endpoint="ep1",micro_path="/path1"} 2`)) {
		// t.Fatal("XXXX")
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
}
