package exporter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/blackbox_exporter/config"
	"github.com/prometheus/blackbox_exporter/prober"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

const (
	configFile     = "config.yml"
	timeoutSeconds = 10
)

var (
	// as specified here https://github.com/GoogleCloudPlatform/buildpacks/blob/56eaad4dfe6c7bd0ecc4a175de030d2cfab9ae1c/cmd/go/functions_framework/main.go#L38.
	sourceCodeDir = "serverless_function_source_code"

	logger  = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	probers = map[string]prober.ProbeFn{
		"http": prober.ProbeHTTP,
		"tcp":  prober.ProbeTCP,
		"icmp": prober.ProbeICMP,
		"dns":  prober.ProbeDNS,
	}
	client *http.Client

	cloudFunctionColdStartGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cloud_function_cold_start",
		Help: "Displays whether or not the cloud function was cold started",
	})
)

// Handler is a http.Handler which will be called by the
// GCP Cloud Function on every request.
// It has been adapted from the original blackbox_exporter probeHandler:
// https://github.com/prometheus/blackbox_exporter/blob/63678419a6a274ac6d43d3d4088cad2a1d06371f/main.go#L70
func Handler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeoutSeconds*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	sc := &config.SafeConfig{
		C: &config.Config{},
	}
	if err := sc.ReloadConfig(path.Join(sourceCodeDir, configFile)); err != nil {
		http.Error(w, fmt.Sprintf("Unable to load config: %s", err), http.StatusInternalServerError)
		return
	}
	moduleName := r.URL.Query().Get("module")
	if moduleName == "" {
		moduleName = "http_2xx"
	}
	module, ok := sc.C.Modules[moduleName]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown module %q", moduleName), http.StatusBadRequest)
		return
	}

	params := r.URL.Query()
	target := params.Get("target")
	if target == "" {
		http.Error(w, "Target parameter is missing", http.StatusBadRequest)
		return
	}

	prober, ok := probers[module.Prober]
	if !ok {
		http.Error(w, fmt.Sprintf("Unknown prober %q", module.Prober), http.StatusBadRequest)
		return
	}

	if module.Prober == "http" {
		if err := coldStartRequest(target); err != nil {
			http.Error(w, fmt.Sprintf("Unable to make a cold start request: %s", err), http.StatusInternalServerError)
			return
		}
	}

	level.Info(logger).Log("msg", "Beginning probe", "probe", module.Prober, "timeout_seconds", timeoutSeconds)

	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Displays whether or not the probe was a success",
	})
	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})

	start := time.Now()
	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)
	registry.MustRegister(cloudFunctionColdStartGauge)
	success := prober(ctx, target, module, registry, logger)
	duration := time.Since(start).Seconds()
	probeDurationGauge.Set(duration)
	if success {
		probeSuccessGauge.Set(1)
		level.Info(logger).Log("msg", "Probe succeeded", "duration_seconds", duration)
	} else {
		level.Error(logger).Log("msg", "Probe failed", "duration_seconds", duration)
	}

	mfs, err := registry.Gather()
	if err != nil {
		level.Debug(logger).Log("msg", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	headers := http.Header{}
	for k, v := range r.Header {
		headers.Add(k, strings.Join(v, ","))
	}
	contentType := expfmt.Negotiate(http.Header(headers))
	buf := &bytes.Buffer{}

	enc := expfmt.NewEncoder(buf, contentType)
	for _, mf := range mfs {
		if err := enc.Encode(mf); err != nil {
			level.Debug(logger).Log("msg", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Add("Content-Type", string(contentType))
	w.Header().Add("Content-Length", fmt.Sprint(buf.Len()))
	fmt.Fprint(w, buf)
}

// If a google cloud function is experiencing a cold start, TLS handshakes were observed
// to sometimes take a really long time just for the first request. This function detects
// cold starts by checking if the global `http.Client` `client` already exists. If not,
// it will issue a GET request to the target so the subsequent http probe will not be
// affected by the cold start.
// https://cloud.google.com/functions/docs/concepts/exec#function_scope_versus_global_scope
func coldStartRequest(target string) error {
	if client != nil {
		cloudFunctionColdStartGauge.Set(0)
		return nil
	}
	cloudFunctionColdStartGauge.Set(1)
	level.Info(logger).Log("msg", "cold start detected, making an initial request to the target", "target", target)
	client = &http.Client{
		Timeout: 10 * time.Second,
	}
	_, err := client.Get(target)
	return err
}
