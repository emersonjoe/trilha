package trilha

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// #307: the upstream reuses its connections under load. http.DefaultTransport
// keeps two idle connections per host, so every burst past the second request
// opened TCP again — the app in front of one API is exactly that one host.
func TestUpstreamKeepsConnectionsUnderLoad(t *testing.T) {
	var opened atomic.Int64
	api := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond) // long enough for a burst to overlap
		w.Write([]byte("ok"))
	}))
	api.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			opened.Add(1)
		}
	}
	api.Start()
	defer api.Close()

	handler := upApp(t, api.URL, nil).Handler()
	const bursts, perBurst = 3, 20
	for b := 0; b < bursts; b++ {
		var wg sync.WaitGroup
		for i := 0; i < perBurst; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/x", nil))
				if rec.Code != 200 {
					t.Errorf("status %d", rec.Code)
				}
			}()
		}
		wg.Wait()
	}
	if n := opened.Load(); n > perBurst+perBurst/2 {
		t.Fatalf("%d connections opened for %d bursts of %d: the pool dropped them between bursts", n, bursts, perBurst)
	}
}

// The transport is one for every upstream and for the app's own API client,
// so the pool is shared, and it is not http.DefaultTransport.
func TestUpstreamTransportIsSharedAndOwn(t *testing.T) {
	tr, ok := UpstreamTransport().(*http.Transport)
	if !ok || tr == http.DefaultTransport || UpstreamTransport() != tr {
		t.Fatalf("UpstreamTransport = %T %p", UpstreamTransport(), UpstreamTransport())
	}
	if tr.MaxIdleConnsPerHost < 32 || tr.IdleConnTimeout <= 0 {
		t.Fatalf("idle per host %d, idle timeout %v", tr.MaxIdleConnsPerHost, tr.IdleConnTimeout)
	}
}
