// +build ignore

package prometheus

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/unistack-org/micro/v3/logger"
	"github.com/unistack-org/micro/v3/metadata"
	"github.com/unistack-org/micro/v3/meter"
)

// Reporter is an implementation of meter.Reporter:
type Reporter struct {
	options            meter.Options
	prometheusRegistry *prometheus.Registry
	metrics            metricFamily
}

// New returns a configured prometheus reporter:
func New(opts ...meter.Option) (meter.Reporter, error) {
	options := meter.NewOptions(opts...)

	// Make a prometheus registry (this keeps track of any metrics we generate):
	prometheusRegistry := prometheus.NewRegistry()
	prometheusRegistry.Register(prometheus.NewGoCollector())
	prometheusRegistry.Register(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{Namespace: "goruntime"}))

	// Make a new Reporter:
	newReporter := &Reporter{
		options:            options,
		prometheusRegistry: prometheusRegistry,
	}

	// Add metrics families for each type:
	newReporter.metrics = newReporter.newMetricFamily()

	// Handle the metrics endpoint with prometheus:
	logger.Infof(options.Context, "Metrics/Prometheus [http] Listening on %s%s", options.Address, options.Path)
	http.Handle(options.Path, promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError}))
	go http.ListenAndServe(options.Address, nil)

	return newReporter, nil
}

func (r *Reporter) Init(...meter.Option) error {
	return nil
}

// convertTags turns Tags into prometheus labels:
func (r *Reporter) convertTags(tags metadata.Metadata) prometheus.Labels {
	labels := prometheus.Labels{}
	for key, value := range tags {
		labels[key] = r.stripUnsupportedCharacters(value)
	}
	return labels
}

// listTagKeys returns a list of tag keys (we need to provide this to the Prometheus client):
func (r *Reporter) listTagKeys(tags metadata.Metadata) (labelKeys []string) {
	for key := range tags {
		labelKeys = append(labelKeys, key)
	}
	return
}

// stripUnsupportedCharacters cleans up a metrics key or value:
func (r *Reporter) stripUnsupportedCharacters(metricName string) string {
	valueWithoutDots := strings.Replace(metricName, ".", "_", -1)
	valueWithoutCommas := strings.Replace(valueWithoutDots, ",", "_", -1)
	valueWIthoutSpaces := strings.Replace(valueWithoutCommas, " ", "", -1)
	return valueWIthoutSpaces
}
