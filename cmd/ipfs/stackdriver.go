package main

import (
	"os"
	"strconv"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/trace"
)

func withStackdriverTracing(f func()) {
	sd, err := stackdriver.NewExporter(stackdriver.Options{
		Location: func() string {
			s, ok := os.LookupEnv("STACKDRIVER_LOCATION")
			if ok {
				return s
			}
			s, _ = os.Hostname()
			return s
		}(),
		ProjectID: os.Getenv("GOOGLE_CLOUD_PROJECT"),
	})
	if err != nil {
		log.Warningf("error creating the stackdriver exporter: %v", err)
		goto noExporter
	}
	// It is imperative to invoke flush before your main function exits
	defer sd.Flush()

	if s, ok := os.LookupEnv("OCTRACE_SAMPLE_PROB"); ok {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			log.Errorf("parsing OC trace sample probability: %v", err)
		} else {
			trace.ApplyConfig(trace.Config{
				DefaultSampler: trace.ProbabilitySampler(f),
			})
		}
	}

	// Register it as a trace exporter
	trace.RegisterExporter(sd)
noExporter:
	f()
}
