package exporter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/blackbox_exporter/config"
	"github.com/prometheus/blackbox_exporter/prober"
	"github.com/prometheus/client_golang/prometheus"
	pconfig "github.com/prometheus/common/config"
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
		if err := overwriteHTTPModuleParams(params, &module.HTTP); err != nil {
			http.Error(w, fmt.Sprintf("Error during parsing module overwrites: %v", err), http.StatusInternalServerError)
			return
		}
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

// overwriteModuleParams allows to specify some overwrites for the http prober:
//
// http_valid_status_codes
// Overwrite the valid HTTP status codes of the request being made by defining comma
// separated status codes. You can also use a shortcut like 2xx for the range of 200-299.
//
// http_expect_regexp
// Define a regular expression which will be matched against the response body. If it matches
// the probe will be marked as successful.
//
// http_fail_on_regexp
// Define a regular expression which will be matched against the response body. If it matches
// the probe will be marked as failed.
//
// http_basic_auth_username
// Define a username which will be used for basic auth in the request being made
//
// http_basic_auth_password
// Define a password which will be used for basic auth in the request being made
func overwriteHTTPModuleParams(params url.Values, conf *config.HTTPProbe) error {
	for name, value := range params {
		switch name {
		case "http_valid_status_codes":
			var validStatusCodes []int
			for _, codes := range value {
				validCodes, err := parseStatusCodes(codes)
				if err != nil {
					return err
				}
				validStatusCodes = append(validStatusCodes, validCodes...)
			}
			conf.ValidStatusCodes = validStatusCodes
		case "http_expect_regexp":
			// we only support one regexp currently
			conf.FailIfBodyNotMatchesRegexp = []string{value[0]}
		case "http_fail_on_regexp":
			// we only support one regexp currently
			conf.FailIfBodyMatchesRegexp = []string{value[0]}
		case "http_basic_auth_username":
			if conf.HTTPClientConfig.BasicAuth == nil {
				conf.HTTPClientConfig.BasicAuth = &pconfig.BasicAuth{}
			}
			conf.HTTPClientConfig.BasicAuth.Username = value[0]
		case "http_basic_auth_password":
			if conf.HTTPClientConfig.BasicAuth == nil {
				conf.HTTPClientConfig.BasicAuth = &pconfig.BasicAuth{}
			}
			conf.HTTPClientConfig.BasicAuth.Password = pconfig.Secret(value[0])
		}
	}
	return nil
}

// parseStatusCodes parses the given comma separated status codes
// it is possible to use shortcuts like 2xx, 3xx, etc which will be expanded
// to the corresponding whole range of status codes
func parseStatusCodes(codes string) ([]int, error) {
	var result []int
	splittedCodes := strings.Split(codes, ",")
	for _, code := range splittedCodes {
		code := strings.TrimSpace(code)
		match, err := regexp.MatchString("[0-9]xx", code)
		if err != nil {
			return nil, err
		}
		if match {
			mainCode, _ := strconv.Atoi(string(code[0]))
			result = append(result, generateCodeRange(mainCode*100, mainCode*100+99)...)
			continue
		}
		parsed, err := strconv.Atoi(code)
		if err != nil {
			return nil, fmt.Errorf("Can not convert status code \"%s\" to a number", code)
		}
		result = append(result, parsed)
	}
	return result, nil
}

// generateCodeRange creates a slice with consecutive ints from start to end
func generateCodeRange(start, end int) []int {
	result := make([]int, end-start+1)
	for i := range result {
		result[i] = start + i
	}
	return result
}
