package corehttp

import (
	"net"
	"net/http"
	"time"

	core "github.com/ipfs/kubo/core"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/zpages"

	ocprom "contrib.go.opencensus.io/exporter/prometheus"
	prometheus "github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsScrapingOption adds the scraping endpoint which Prometheus uses to fetch metrics.
func MetricsScrapingOption(path string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle(path, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
		return mux, nil
	}
}

// This adds collection of OpenCensus metrics
func MetricsOpenCensusCollectionOption() ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		log.Info("Init OpenCensus")

		promRegistry := prometheus.NewRegistry()
		pe, err := ocprom.NewExporter(ocprom.Options{
			Namespace: "ipfs_oc",
			Registry:  promRegistry,
			OnError: func(err error) {
				log.Errorw("OC ERROR", "error", err)
			},
		})
		if err != nil {
			return nil, err
		}

		// register prometheus with opencensus
		view.RegisterExporter(pe)
		view.SetReportingPeriod(2 * time.Second)

		// Construct the mux
		zpages.Handle(mux, "/debug/metrics/oc/debugz")
		mux.Handle("/debug/metrics/oc", pe)

		return mux, nil
	}
}

// MetricsOpenCensusDefaultPrometheusRegistry registers the default prometheus
// registry as an exporter to OpenCensus metrics. This means that OpenCensus
// metrics will show up in the prometheus metrics endpoint
func MetricsOpenCensusDefaultPrometheusRegistry() ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		log.Info("Init OpenCensus with default prometheus registry")

		pe, err := ocprom.NewExporter(ocprom.Options{
			Registry: prometheus.DefaultRegisterer.(*prometheus.Registry),
			OnError: func(err error) {
				log.Errorw("OC default registry ERROR", "error", err)
			},
		})
		if err != nil {
			return nil, err
		}

		// register prometheus with opencensus
		view.RegisterExporter(pe)

		return mux, nil
	}
}

// MetricsCollectionOption adds collection of net/http-related metrics.
func MetricsCollectionOption(handlerName string) ServeOption {
	return func(_ *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		// Adapted from github.com/prometheus/client_golang/prometheus/http.go
		// Work around https://github.com/prometheus/client_golang/pull/311
		opts := prometheus.SummaryOpts{
			Namespace:   "ipfs",
			Subsystem:   "http",
			ConstLabels: prometheus.Labels{"handler": handlerName},
			Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		}

		reqCnt := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace:   opts.Namespace,
				Subsystem:   opts.Subsystem,
				Name:        "requests_total",
				Help:        "Total number of HTTP requests made.",
				ConstLabels: opts.ConstLabels,
			},
			[]string{"method", "code"},
		)
		if err := prometheus.Register(reqCnt); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				reqCnt = are.ExistingCollector.(*prometheus.CounterVec)
			} else {
				return nil, err
			}
		}

		opts.Name = "request_duration_seconds"
		opts.Help = "The HTTP request latencies in seconds."
		reqDur := prometheus.NewSummaryVec(opts, nil)
		if err := prometheus.Register(reqDur); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				reqDur = are.ExistingCollector.(*prometheus.SummaryVec)
			} else {
				return nil, err
			}
		}

		opts.Name = "request_size_bytes"
		opts.Help = "The HTTP request sizes in bytes."
		reqSz := prometheus.NewSummaryVec(opts, nil)
		if err := prometheus.Register(reqSz); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				reqSz = are.ExistingCollector.(*prometheus.SummaryVec)
			} else {
				return nil, err
			}
		}

		opts.Name = "response_size_bytes"
		opts.Help = "The HTTP response sizes in bytes."
		resSz := prometheus.NewSummaryVec(opts, nil)
		if err := prometheus.Register(resSz); err != nil {
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				resSz = are.ExistingCollector.(*prometheus.SummaryVec)
			} else {
				return nil, err
			}
		}

		// Construct the mux
		childMux := http.NewServeMux()
		var promMux http.Handler = childMux
		promMux = promhttp.InstrumentHandlerResponseSize(resSz, promMux)
		promMux = promhttp.InstrumentHandlerRequestSize(reqSz, promMux)
		promMux = promhttp.InstrumentHandlerDuration(reqDur, promMux)
		promMux = promhttp.InstrumentHandlerCounter(reqCnt, promMux)
		mux.Handle("/", promMux)

		return childMux, nil
	}
}

var peersTotalMetric = prometheus.NewDesc(
	prometheus.BuildFQName("ipfs", "p2p", "peers_total"),
	"Number of connected peers",
	[]string{"transport"},
	nil,
)

type IpfsNodeCollector struct {
	Node *core.IpfsNode
}

func (IpfsNodeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- peersTotalMetric
}

func (c IpfsNodeCollector) Collect(ch chan<- prometheus.Metric) {
	for tr, val := range c.PeersTotalValues() {
		ch <- prometheus.MustNewConstMetric(
			peersTotalMetric,
			prometheus.GaugeValue,
			val,
			tr,
		)
	}
}

func (c IpfsNodeCollector) PeersTotalValues() map[string]float64 {
	vals := make(map[string]float64)
	if c.Node.PeerHost == nil {
		return vals
	}
	for _, peerID := range c.Node.PeerHost.Network().Peers() {
		// Each peer may have more than one connection (see for an explanation
		// https://github.com/libp2p/go-libp2p-swarm/commit/0538806), so we grab
		// only one, the first (an arbitrary and non-deterministic choice), which
		// according to ConnsToPeer is the oldest connection in the list
		// (https://github.com/libp2p/go-libp2p-swarm/blob/v0.2.6/swarm.go#L362-L364).
		conns := c.Node.PeerHost.Network().ConnsToPeer(peerID)
		if len(conns) == 0 {
			continue
		}
		tr := ""
		for _, proto := range conns[0].RemoteMultiaddr().Protocols() {
			tr = tr + "/" + proto.Name
		}
		vals[tr] = vals[tr] + 1
	}
	return vals
}
