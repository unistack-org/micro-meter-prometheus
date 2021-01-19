// +build ignore

package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unistack-org/micro/v3/meter"
)

func TestOptions(t *testing.T) {
	// Make some new options:
	options := meter.NewOptions(meter.Path("/prometheus"), meter.DefaultTags(map[string]string{"service": "prometheus-test"}))

	// Check that the defaults and overrides were accepted:
	assert.Equal(t, ":9000", options.Address)
	assert.Equal(t, "/prometheus", options.Path)
	assert.Equal(t, "prometheus-test", options.DefaultTags["service"])
}
