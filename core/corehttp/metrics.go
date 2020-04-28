package corehttp

import (
	"net"
	"net/http"

	core "github.com/ipfs/go-ipfs/core"

	prometheus "github.com/prometheus/client_golang/prometheus"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
)

// This adds the scraping endpoint which Prometheus uses to fetch metrics.
func MetricsScrapingOption(path string) ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle(path, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
		return mux, nil
	}
}

// This adds collection of net/http-related metrics
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

var (
	peersTotalMetric = prometheus.NewDesc(
		prometheus.BuildFQName("ipfs", "p2p", "peers_total"),
		"Number of connected peers", []string{"transport"}, nil)

	unixfsGetMetric = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "ipfs",
		Subsystem: "http",
		Name:      "unixfs_get_latency_seconds",
		Help:      "The time till the first block is received when 'getting' a file from the gateway.",
	}, []string{"namespace"})
)

type IpfsNodeCollector struct {
	Node *core.IpfsNode
}

func (_ IpfsNodeCollector) Describe(ch chan<- *prometheus.Desc) {
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
	for _, conn := range c.Node.PeerHost.Network().Conns() {
		tr := ""
		for _, proto := range conn.RemoteMultiaddr().Protocols() {
			tr = tr + "/" + proto.Name
		}
		vals[tr] = vals[tr] + 1
	}
	return vals
}
