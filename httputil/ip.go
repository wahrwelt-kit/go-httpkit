package httputil

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// ParseTrustedProxyCIDRs parses CIDR strings into networks. Invalid or empty entries are skipped.
// Returns (nil, error) if all entries are invalid. If only some are invalid, returns (valid nets, error) so the caller can log the invalid list.
func ParseTrustedProxyCIDRs(cidrs []string) ([]*net.IPNet, error) {
	if len(cidrs) == 0 {
		return nil, nil
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	var invalid []string
	for _, cidrStr := range cidrs {
		cidrStr = strings.TrimSpace(cidrStr)
		if cidrStr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidrStr)
		if err != nil {
			invalid = append(invalid, cidrStr)
			continue
		}
		nets = append(nets, network)
	}
	if len(nets) == 0 {
		return nil, fmt.Errorf("httputil: no valid trusted proxy CIDRs")
	}
	if len(invalid) > 0 {
		return nets, fmt.Errorf("httputil: invalid trusted proxy CIDRs skipped: %v", invalid)
	}
	return nets, nil
}

// GetClientIPWithNets returns the client IP, using X-Real-IP and X-Forwarded-For when RemoteAddr is in trustedNets.
func GetClientIPWithNets(r *http.Request, trustedNets []*net.IPNet) string {
	if r == nil {
		return ""
	}
	remoteIP := peerIP(r.RemoteAddr)
	if len(trustedNets) == 0 {
		return remoteIP
	}
	if !isIPInNets(remoteIP, trustedNets) {
		return remoteIP
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		ipStr := strings.TrimSpace(strings.Split(ip, ",")[0])
		if parsed := net.ParseIP(ipStr); parsed != nil && !isIPInNets(ipStr, trustedNets) {
			return ipStr
		}
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ipStr := strings.TrimSpace(parts[i])
			if ipStr == "" || net.ParseIP(ipStr) == nil {
				continue
			}
			if !isIPInNets(ipStr, trustedNets) {
				return ipStr
			}
		}
	}
	return remoteIP
}

// GetClientIP returns the client IP. When RemoteAddr is in a trusted proxy CIDR, X-Real-IP and X-Forwarded-For are used.
// Parse errors from trustedProxyCIDRs are ignored; if all are invalid, only RemoteAddr is used.
//
// Deprecated: Use GetClientIPE so invalid CIDR configuration is not silently ignored.
func GetClientIP(r *http.Request, trustedProxyCIDRs []string) string {
	nets, _ := ParseTrustedProxyCIDRs(trustedProxyCIDRs)
	return GetClientIPWithNets(r, nets)
}

// GetClientIPE returns the client IP like GetClientIP but returns an error when ParseTrustedProxyCIDRs fails (e.g. all CIDRs invalid). When only some CIDRs are invalid, returns the IP using valid nets and the parse error so the caller can log it.
func GetClientIPE(r *http.Request, trustedProxyCIDRs []string) (string, error) {
	nets, err := ParseTrustedProxyCIDRs(trustedProxyCIDRs)
	if err != nil && nets == nil {
		return "", err
	}
	return GetClientIPWithNets(r, nets), err
}

// peerIP returns the IP address from a remote address string.
func peerIP(remoteAddr string) string {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return ip
}

// isIPInNets checks if an IP address is in a list of networks.
func isIPInNets(ipStr string, nets []*net.IPNet) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, network := range nets {
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
