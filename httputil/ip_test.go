package httputil

import (
	"net/http"
	"testing"
)

const (
	testCIDR10       = "10.0.0.0/8"
	testIP192        = "192.168.1.1"
	testIP10001      = "10.0.0.1"
	testIPPublic1    = "203.0.113.1"
	testRemoteAddr10 = "10.0.0.2:80"
)

func TestParseTrustedProxyCIDRs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		cidrs      []string
		wantErr    bool
		wantNets   int
		wantErrAny bool
	}{
		{"empty", nil, false, 0, false},
		{"empty slice", []string{}, false, 0, false},
		{"all invalid", []string{"bad", "10.0.0.0/33"}, true, 0, false},
		{"one valid", []string{testCIDR10}, false, 1, false},
		{"mixed", []string{"bad", testCIDR10, "192.168.0.0/16"}, false, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nets, err := ParseTrustedProxyCIDRs(tt.cidrs)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if tt.wantErrAny {
				if err == nil {
					t.Error("expected error for invalid entries")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tt.wantNets > 0 && len(nets) != tt.wantNets {
				t.Errorf("len(nets) = %d, want %d", len(nets), tt.wantNets)
			}
		})
	}
}

func TestGetClientIPE(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		remoteAddr        string
		headers           map[string]string
		trustedProxyCIDRs []string
		want              string
	}{
		{"no proxy", testIP192 + ":12345", nil, nil, testIP192},
		{"no trusted CIDRs ignores headers", testIP192 + ":12345", map[string]string{headerXRealIP: testIP10001}, nil, testIP192},
		{"trusted proxy X-Real-IP", testRemoteAddr10, map[string]string{headerXRealIP: testIPPublic1}, []string{testCIDR10}, testIPPublic1},
		{"trusted proxy X-Forwarded-For", testRemoteAddr10, map[string]string{headerXForwardedFor: "203.0.113.2"}, []string{testCIDR10}, "203.0.113.2"},
		{"X-Real-IP preferred over X-Forwarded-For", testRemoteAddr10, map[string]string{headerXRealIP: "1.2.3.4", headerXForwardedFor: "5.6.7.8"}, []string{testCIDR10}, "1.2.3.4"},
		{"untrusted proxy uses remote", testIP192 + ":80", map[string]string{headerXRealIP: testIP10001}, []string{testCIDR10}, testIP192},
		{"rightmost non-trusted from X-Forwarded-For", testRemoteAddr10, map[string]string{headerXForwardedFor: " 203.0.113.3 , 10.0.0.1 "}, []string{testCIDR10}, "203.0.113.3"},
		{"X-Forwarded-For spoofing: take rightmost", testRemoteAddr10, map[string]string{headerXForwardedFor: "1.2.3.4, 203.0.113.50"}, []string{testCIDR10}, "203.0.113.50"},
		{"X-Real-IP in trusted network is ignored", testRemoteAddr10, map[string]string{headerXRealIP: testIP10001, headerXForwardedFor: testIPPublic1}, []string{testCIDR10}, testIPPublic1},
		{"X-Real-IP only and in trusted uses remote", testRemoteAddr10, map[string]string{headerXRealIP: testIP10001}, []string{testCIDR10}, "10.0.0.2"},
	}
	for _, tt := range tests {
		r, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
		r.RemoteAddr = tt.remoteAddr
		for k, v := range tt.headers {
			r.Header.Set(k, v)
		}
		got, _ := GetClientIPE(r, tt.trustedProxyCIDRs)
		if got != tt.want {
			t.Errorf("%s: GetClientIPE() = %q, want %q", tt.name, got, tt.want)
		}
	}
}
