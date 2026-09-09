package webhook

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/emersonjoe/trilha"
)

// check refuses an address a webhook must not reach.
//
// This is the part nobody writes on their own, and the reason is that the
// dangerous case does not look dangerous. The URL belongs to a partner, but
// the person typing it works here, and a form that accepts
// http://169.254.169.254/latest/meta-data/ turns your own server into the
// thing that fetches the machine's cloud credentials and posts them to
// whoever asked. The same trick reaches an internal admin panel on
// 10.0.0.5, or a database on localhost.
//
// So: https only, except in dev; and every address the name resolves to has
// to be a public one. Every address, because a name that answers with one
// public IP and one loopback address would otherwise pass.
func (h *Hooks) check(raw string) error {
	if h.allow {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("webhook: %q is not a URL: %w", raw, err)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if h.env != trilha.Dev {
			return fmt.Errorf("webhook: %s is http; a webhook carries a signature and a payload over somebody else's network, so https is the only option outside dev", raw)
		}
	default:
		return fmt.Errorf("webhook: %q needs http:// or https://", raw)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook: %q has no host", raw)
	}
	ips, err := h.lookup(host)
	if err != nil {
		return fmt.Errorf("webhook: %s does not resolve: %w", host, err)
	}
	for _, ip := range ips {
		reason, local := private(ip)
		if reason == "" {
			continue
		}
		// In dev the partner is on this machine — a receiver on localhost:4000
		// is how anybody tries this the first time, and a check that refuses it
		// is a check that gets turned off wholesale.
		if local && h.env == trilha.Dev {
			continue
		}
		return fmt.Errorf("webhook: %s resolves to %s, which is %s — a webhook may only reach the public internet", host, ip, reason)
	}
	return nil
}

// private names why an address is out of bounds, and whether it is the kind of
// out of bounds that development legitimately needs.
//
// The name matters: "is a private network address" is something the person who
// typed the URL can act on, and "invalid" is not.
//
// The second answer is the line between "your partner runs on your laptop",
// which is normal in dev, and "your server is being asked to read its own
// cloud credentials", which is never anybody's development setup. Link-local
// is refused in every environment for that reason: a dev config that gets
// promoted is exactly how that one bites.
func private(ip net.IP) (reason string, devOK bool) {
	switch {
	case ip.IsLoopback():
		return "this machine", true
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast():
		// 169.254.0.0/16 is where every cloud keeps its metadata service, and
		// it is the address this whole function exists for.
		return "link-local, where cloud metadata services live", false
	case ip.IsPrivate():
		return "a private network", true
	case ip.IsUnspecified():
		return "unspecified", false
	case ip.IsMulticast(), ip.IsInterfaceLocalMulticast():
		return "multicast", false
	case cgnat(ip):
		return "carrier-grade NAT", true
	case v4(ip) != nil && v4(ip)[0] == 0:
		return "reserved", false
	}
	return "", false
}

// cgnat is 100.64.0.0/10, which net.IP has no method for and which is where a
// good part of the internal traffic of a cloud actually sits.
func cgnat(ip net.IP) bool {
	b := v4(ip)
	return b != nil && b[0] == 100 && b[1] >= 64 && b[1] <= 127
}

func v4(ip net.IP) net.IP { return ip.To4() }

// lookup resolves a name, and is a field so the tests can answer without a
// network — including the case a single lookup cannot show by accident: a name
// that answers with one public address and one loopback address, which passes
// any check that looks at only the first.
func (h *Hooks) lookup(host string) ([]net.IP, error) {
	if h.resolve != nil {
		return h.resolve(host)
	}
	return net.LookupIP(host)
}
