package exporter

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alecthomas/assert"
)

var ts *httptest.Server

func TestMain(m *testing.M) {
	// override sourceCodeDir for test
	sourceCodeDir = ""
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	}))
	code := m.Run()
	ts.Close()
	os.Exit(code)
}

func TestHandler(t *testing.T) {
	handlerTests := []struct {
		name           string
		target         string
		module         string
		bodyContains   string
		expectedStatus int
	}{
		{"valid target", ts.URL, "http_2xx", "probe_success 1", http.StatusOK},
		{"default module", ts.URL, "", "probe_success 1", http.StatusOK},
		{"invalid module", ts.URL, "http_invalid", "Unknown module", http.StatusBadRequest},
		{"invalid target", "http://doesnot.exist.local", "http_2xx", "probe_success 0", http.StatusOK},
		{"no target", "", "http_2xx", "Target parameter is missing", http.StatusBadRequest},
	}
	for _, tt := range handlerTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://exporter/probe?target="+tt.target+"&module="+tt.module, nil)
			w := httptest.NewRecorder()
			Handler(w, req)
			resp := w.Result()
			body, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			assert.Contains(t, string(body), tt.bodyContains)
		})
	}
}
