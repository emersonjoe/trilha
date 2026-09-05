package trilha

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// parseProxies turns CIDR/IP strings into prefixes; invalid entries are
// ignored with a log line.
func (a *App) parseProxies() {
	for _, s := range a.cfg.TrustedProxies {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			if ip, err := netip.ParseAddr(s); err == nil {
				s = netip.PrefixFrom(ip, ip.BitLen()).String()
			}
		}
		p, err := netip.ParsePrefix(s)
		if err != nil {
			a.log.Warn("trilha: TrustedProxies entry ignored", "entry", s, "err", err)
			continue
		}
		a.proxies = append(a.proxies, p)
	}
}

func (a *App) trusted(ip netip.Addr) bool {
	for _, p := range a.proxies {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

func remoteAddr(s string) netip.Addr {
	host, _, err := net.SplitHostPort(s)
	if err != nil {
		host = s
	}
	ip, _ := netip.ParseAddr(host)
	return ip.Unmap()
}

// fromTrustedProxy reports whether the direct peer is a trusted proxy.
func (c *Ctx) fromTrustedProxy() bool {
	return c.app.trusted(remoteAddr(c.r.RemoteAddr))
}

// ClientIP returns the client address: RemoteAddr, or the right-most
// untrusted entry of X-Forwarded-For when the peer is a trusted proxy.
func (c *Ctx) ClientIP() string { return clientIPOf(c.app, c.r) }

// clientIPOf is ClientIP for code that has no Ctx (probe endpoints).
func clientIPOf(a *App, r *http.Request) string {
	peer := remoteAddr(r.RemoteAddr)
	if !a.trusted(peer) {
		return peer.String()
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			continue
		}
		if !a.trusted(ip) {
			return ip.Unmap().String()
		}
	}
	return peer.String()
}

// isSecure reports HTTPS: direct TLS, or a trusted proxy saying so.
func (c *Ctx) isSecure() bool {
	if c.r.TLS != nil {
		return true
	}
	return c.fromTrustedProxy() && strings.EqualFold(c.r.Header.Get("X-Forwarded-Proto"), "https")
}
