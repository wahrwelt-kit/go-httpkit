package httputil

import (
	"net/http"
	"testing"
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
		{"one valid", []string{"10.0.0.0/8"}, false, 1, false},
		{"mixed", []string{"bad", "10.0.0.0/8", "192.168.0.0/16"}, false, 2, true},
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

func TestGetClientIP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		remoteAddr        string
		headers           map[string]string
		trustedProxyCIDRs []string
		want              string
	}{
		{"no proxy", "192.168.1.1:12345", nil, nil, "192.168.1.1"},
		{"no trusted CIDRs ignores headers", "192.168.1.1:12345", map[string]string{"X-Real-IP": "10.0.0.1"}, nil, "192.168.1.1"},
		{"trusted proxy X-Real-IP", "10.0.0.2:80", map[string]string{"X-Real-IP": "203.0.113.1"}, []string{"10.0.0.0/8"}, "203.0.113.1"},
		{"trusted proxy X-Forwarded-For", "10.0.0.2:80", map[string]string{"X-Forwarded-For": "203.0.113.2"}, []string{"10.0.0.0/8"}, "203.0.113.2"},
		{"X-Real-IP preferred over X-Forwarded-For", "10.0.0.2:80", map[string]string{"X-Real-IP": "1.2.3.4", "X-Forwarded-For": "5.6.7.8"}, []string{"10.0.0.0/8"}, "1.2.3.4"},
		{"untrusted proxy uses remote", "192.168.1.1:80", map[string]string{"X-Real-IP": "10.0.0.1"}, []string{"10.0.0.0/8"}, "192.168.1.1"},
		{"rightmost non-trusted from X-Forwarded-For", "10.0.0.2:80", map[string]string{"X-Forwarded-For": " 203.0.113.3 , 10.0.0.1 "}, []string{"10.0.0.0/8"}, "203.0.113.3"},
		{"X-Forwarded-For spoofing: take rightmost", "10.0.0.2:80", map[string]string{"X-Forwarded-For": "1.2.3.4, 203.0.113.50"}, []string{"10.0.0.0/8"}, "203.0.113.50"},
		{"X-Real-IP in trusted network is ignored", "10.0.0.2:80", map[string]string{"X-Real-IP": "10.0.0.1", "X-Forwarded-For": "203.0.113.1"}, []string{"10.0.0.0/8"}, "203.0.113.1"},
		{"X-Real-IP only and in trusted uses remote", "10.0.0.2:80", map[string]string{"X-Real-IP": "10.0.0.1"}, []string{"10.0.0.0/8"}, "10.0.0.2"},
	}
	for _, tt := range tests {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = tt.remoteAddr
		for k, v := range tt.headers {
			r.Header.Set(k, v)
		}
		got := GetClientIP(r, tt.trustedProxyCIDRs)
		if got != tt.want {
			t.Errorf("%s: GetClientIP() = %q, want %q", tt.name, got, tt.want)
		}
	}
}
