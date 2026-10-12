package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMiddlewareBoundsMethodCardinality(t *testing.T) {
	registry := prometheus.NewRegistry()
	var received string
	handler := Middleware(registry, nil, WithInFlight())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Method
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	for i := range 100 {
		method := fmt.Sprintf("CUSTOM%d", i)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/", http.NoBody))
		if received != method {
			t.Fatalf("request method changed to %q", received)
		}
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 3 {
		t.Fatalf("metric families: %d", len(families))
	}
	for _, family := range families {
		if len(family.GetMetric()) != 1 {
			t.Errorf("%s retained %d series", family.GetName(), len(family.GetMetric()))
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == keyMethod && label.GetValue() != "unknown" {
					t.Errorf("method: %q", label.GetValue())
				}
			}
		}
	}
}

func TestMetricMethodKeepsStandardMethods(t *testing.T) {
	for _, method := range []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace, http.MethodPatch,
	} {
		if got := metricMethod(method); got != method {
			t.Errorf("%s became %s", method, got)
		}
	}
}
