package prometheus

import (
	"bytes"
	"fmt"
	"testing"

	"go.unistack.org/micro/v4/meter"
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
		// t.Fatal("XXXX")
		t.Fatalf("invalid metrics output: %s", buf.Bytes())
	}
}

func TestCounterSet(t *testing.T) {
	m := NewMeter()

	value := uint64(42)

	m.Counter("forte_accounts_total", "channel_code", "crm").Set(value)

	fmt.Println(uint64(float64(value)))

	buf := bytes.NewBuffer(nil)

	_ = m.Write(buf)

	output := buf.String()

	fmt.Println(output)

	expectedOutput := fmt.Sprintf(`%s{channel_code="crm"} %d`, "forte_accounts_total", value)

	if !bytes.Contains(buf.Bytes(), []byte(expectedOutput)) {
		t.Fatalf("invalid metrics output: expected %q, got %q", expectedOutput, output)
	}
}
