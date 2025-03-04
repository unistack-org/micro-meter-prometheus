package prometheus

import (
	"bytes"
	"fmt"
	"testing"

	"go.unistack.org/micro/v3/meter"
)

func TestBuildMetric(t *testing.T) {
	m := NewMeter(meter.Labels("service_name", "test", "service_version", "0.0.0.1"))
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}

	name := m.buildMetric("micro_server")
	if name != `micro_server{service_name="test",service_version="0.0.0.1"}` {
		t.Fatal("invalid name")
	}
}

func TestWithDefaultLabels(t *testing.T) {
	m := NewMeter(meter.Labels("service_name", "test", "service_version", "0.0.0.1"))
	if err := m.Init(); err != nil {
		t.Fatal(err)
	}
	m.Counter("micro_server", "endpoint", "ep3", "path", "/path3", "status", "success").Inc()

	buf := bytes.NewBuffer(nil)
	_ = m.Write(buf, meter.WriteProcessMetrics(false), meter.WriteFDMetrics(false))
	if !bytes.Contains(buf.Bytes(), []byte(`micro_server{service_name="test",service_version="0.0.0.1",endpoint="ep3",path="/path3",status="success"} 1`)) {
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
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

func TestBuildName(t *testing.T) {
	m := NewMeter()
	check := `micro_foo{aaa="b",bar="baz",ccc="d"}`
	name := m.buildMetric("micro_foo", "bar", "baz", "aaa", "b", "ccc", "d")
	if name != check {
		t.Fatalf("metric name error: %s != %s", name, check)
	}

	cnt := m.Counter("counter", "key", "val")
	cnt.Inc()
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
}

func TestCounterSet(t *testing.T) {
	m := NewMeter()

	value := uint64(42)

	m.Counter("forte_accounts_total", "channel_code", "crm").Set(value)

	buf := bytes.NewBuffer(nil)

	_ = m.Write(buf)

	output := buf.String()

	expectedOutput := fmt.Sprintf(`%s{channel_code="crm"} %d`, "forte_accounts_total", value)

	if !bytes.Contains(buf.Bytes(), []byte(expectedOutput)) {
		t.Fatalf("invalid metrics output: expected %q, got %q", expectedOutput, output)
	}
}
