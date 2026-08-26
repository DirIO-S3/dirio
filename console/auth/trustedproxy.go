package auth

import (
	"fmt"
	"net"
	"strings"
)

// TrustedProxies is an allowlist of proxy IP addresses/CIDR ranges that are
// trusted to sit directly in front of the console and report request
// metadata via Forwarded-family headers (X-Forwarded-Proto today; the same
// allowlist is meant to gate any other X-Forwarded-* / Forwarded value the
// console learns to consume later — e.g. X-Forwarded-For for client IP —
// rather than each header growing its own trust check). Only requests whose
// immediate peer (http.Request.RemoteAddr) matches an entry are trusted;
// forwarded headers from anyone else are ignored. A nil *TrustedProxies (or
// one parsed from an empty list) trusts nothing, so forwarded headers are
// always ignored by default.
type TrustedProxies struct {
	nets []*net.IPNet
	ips  []net.IP
}

// ParseTrustedProxies parses a comma-separated list of IPs and/or CIDR
// ranges (e.g. "10.0.0.0/8,192.168.1.5") identifying reverse proxies/ingress
// controllers allowed to set forwarded-header information. An empty or
// blank list yields a TrustedProxies that trusts nothing.
//
// This single list is the source of truth for every Forwarded-family header
// the console trusts — do not add a second, header-specific allowlist.
func ParseTrustedProxies(csv string) (*TrustedProxies, error) {
	tp := &TrustedProxies{}
	for _, entry := range strings.Split(csv, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", entry, err)
			}
			tp.nets = append(tp.nets, ipNet)
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, fmt.Errorf("invalid trusted proxy IP %q", entry)
		}
		tp.ips = append(tp.ips, ip)
	}
	return tp, nil
}

// Contains reports whether remoteAddr (a "host:port" or bare host, as found
// on http.Request.RemoteAddr) matches an entry in the allowlist.
func (tp *TrustedProxies) Contains(remoteAddr string) bool {
	if tp == nil {
		return false
	}
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, known := range tp.ips {
		if known.Equal(ip) {
			return true
		}
	}
	for _, n := range tp.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
