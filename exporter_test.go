package exporter

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/alecthomas/assert"
)

var ts map[string]*httptest.Server

func TestMain(m *testing.M) {
	// override sourceCodeDir for test
	sourceCodeDir = ""
	ts = initTestServers()
	code := m.Run()
	for _, server := range ts {
		server.Close()
	}
	os.Exit(code)
}

func initTestServers() map[string]*httptest.Server {
	result := make(map[string]*httptest.Server)
	result["simple"] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	}))
	result["randomClientError"] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rand.Seed(time.Now().UnixNano())
		http.Error(w, "client error", 400+rand.Intn(100))
	}))
	result["serverError"] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rand.Seed(time.Now().UnixNano())
		errorCodes := []int{500, 501, 502, 503}
		http.Error(w, "server error", errorCodes[rand.Intn(len(errorCodes))])
	}))
	return result
}

func TestHandler(t *testing.T) {
	handlerTests := []struct {
		name           string
		target         string
		module         string
		overwrites     func(t *testing.T) url.Values
		bodyContains   string
		expectedStatus int
	}{
		{"valid target", ts["simple"].URL, "http_2xx", urlValue(), "probe_success 1", http.StatusOK},
		{"default module", ts["simple"].URL, "", urlValue(), "probe_success 1", http.StatusOK},
		{"invalid module", ts["simple"].URL, "http_invalid", urlValue(), "Unknown module", http.StatusBadRequest},
		{"invalid target", "http://doesnot.exist.local", "http_2xx", urlValue(), "probe_success 0", http.StatusOK},
		{"no target", "", "http_2xx", urlValue(), "Target parameter is missing", http.StatusBadRequest},
		{"simple status code overwrite", ts["simple"].URL, "", urlValue("http_valid_status_codes", "200"), "probe_success 1", http.StatusOK},
		{"special notation status code", ts["randomClientError"].URL, "", urlValue("http_valid_status_codes", "4xx"), "probe_success 1", http.StatusOK},
		{"comma separated codes", ts["serverError"].URL, "", urlValue("http_valid_status_codes", "500,501,502,503"), "probe_success 1", http.StatusOK},
		{"white space comma separated codes", ts["serverError"].URL, "", urlValue("http_valid_status_codes", "500, 501, 502, 503"), "probe_success 1", http.StatusOK},
		{"positive expect regexp test", ts["simple"].URL, "", urlValue("http_expect_regexp", "[oO][kK]"), "probe_success 1", http.StatusOK},
		{"negative expect regexp test", ts["simple"].URL, "", urlValue("http_expect_regexp", "fail"), "probe_success 0", http.StatusOK},
		{"empty expect regexp test", ts["simple"].URL, "", urlValue("http_expect_regexp", ""), "probe_success 1", http.StatusOK},
		{"invalid expect regexp test", ts["simple"].URL, "", urlValue("http_expect_regexp", "*"), "Error during parsing module overwrites", http.StatusBadRequest},
		{"positive fail on regexp test", ts["simple"].URL, "", urlValue("http_fail_on_regexp", "[oO][kK]"), "probe_success 0", http.StatusOK},
		{"negative fail on regexp test", ts["simple"].URL, "", urlValue("http_fail_on_regexp", "fail"), "probe_success 1", http.StatusOK},
		{"empty fail on regexp test", ts["simple"].URL, "", urlValue("http_expect_regexp", ""), "probe_success 1", http.StatusOK},
		{"confusing regexp test", ts["simple"].URL, "", urlValue("http_expect_regexp", "ok", "http_fail_on_regexp", "ok"), "probe_success 0", http.StatusOK},
	}
	for _, tt := range handlerTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			testURL := "http://exporter/probe?target=" + tt.target + "&module=" + tt.module
			if len(tt.overwrites(t)) > 0 {
				testURL = testURL + "&" + tt.overwrites(t).Encode()
			}
			t.Logf("Using URL: %s\n", testURL)
			req := httptest.NewRequest("GET", testURL, nil)
			w := httptest.NewRecorder()
			Handler(w, req)
			resp := w.Result()
			body, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode, fmt.Sprintf("Expected status code %v, but got %v", tt.expectedStatus, resp.StatusCode))
			assert.Contains(t, string(body), tt.bodyContains)
		})
	}
}

func urlValue(keyValue ...string) func(t *testing.T) url.Values {
	return func(t *testing.T) url.Values {
		if len(keyValue)%2 != 0 {
			t.Fatal("Expected key-value pairs as arguments, but got uneven amount")
		}
		uv := url.Values{}
		for i := 0; i < len(keyValue); i = i + 2 {
			uv.Add(keyValue[i], keyValue[i+1])
		}
		return uv
	}
}
