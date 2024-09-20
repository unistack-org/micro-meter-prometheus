package prometheus

import (
	"bytes"
	"context"
	"testing"

	"go.unistack.org/micro/v3/client"
	"go.unistack.org/micro/v3/codec"
	"go.unistack.org/micro/v3/meter"
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
		// t.Fatal("XXXX")
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
}
