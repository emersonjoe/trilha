package trilha

import (
	"net/http"
	"sync"
	"time"
)

// upstreamTransport is built once: one pool for every upstream and for the
// app's own client of the same API.
var upstreamTransport = sync.OnceValue(func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 256
	t.MaxIdleConnsPerHost = 64
	t.IdleConnTimeout = 90 * time.Second
	return t
})

// UpstreamTransport is the transport Config.Upstreams uses when Transport is
// nil, sized for an app in front of one API: 64 idle connections kept per
// host, where http.DefaultTransport keeps 2 and opens TCP (and TLS) again for
// every request past the second of a burst (#307).
//
// The app's own client of that API should share it, with a timeout —
// http.DefaultClient has neither the pool nor the timeout:
//
//	var api = &http.Client{Transport: trilha.UpstreamTransport(), Timeout: 10 * time.Second}
func UpstreamTransport() http.RoundTripper { return upstreamTransport() }
