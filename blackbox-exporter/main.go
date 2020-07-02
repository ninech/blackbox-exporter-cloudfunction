package exporter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/blackbox_exporter/config"
	"github.com/prometheus/blackbox_exporter/prober"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	yaml "gopkg.in/yaml.v2"
)

var (
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
)

func Handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	proberp := r.URL.Path
	if len(proberp) < 1 {
		http.Error(w, "Prober not set", http.StatusBadRequest)
		return
	}
	target := r.FormValue("target")
	if target == "" {
		http.Error(w, "Query parameter target is not set", http.StatusBadRequest)
		return
	}
	cfg := r.FormValue("config")
	if cfg == "" {
		http.Error(w, "Query parameter config is not set", http.StatusBadRequest)
		return
	}
	proberp = proberp[1:]
	logger := log.With(logger, "target", target, "prober", proberp)
	level.Info(logger).Log("msg", "Got request")

	registry := prometheus.NewRegistry()

	module := config.Module{}
	switch proberp {
	case "http":
		if err := yaml.Unmarshal([]byte(cfg), &module.HTTP); err != nil {
			level.Debug(logger).Log("msg", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prober.ProbeHTTP(ctx, target, module, registry, logger)
	case "tcp":
		if err := yaml.Unmarshal([]byte(cfg), &module.TCP); err != nil {
			level.Debug(logger).Log("msg", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prober.ProbeTCP(ctx, target, module, registry, logger)
	case "dns":
		if err := yaml.Unmarshal([]byte(cfg), &module.DNS); err != nil {
			level.Debug(logger).Log("msg", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prober.ProbeDNS(ctx, target, module, registry, logger)
	case "icmp":
		if err := yaml.Unmarshal([]byte(cfg), &module.ICMP); err != nil {
			level.Debug(logger).Log("msg", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prober.ProbeICMP(ctx, target, module, registry, logger)
	default:
		http.Error(w, "invalid probe", http.StatusBadRequest)
		return
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
