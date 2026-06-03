package server

import (
	"html/template"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MaxMemSamples is how many recent readings the chart history holds. At the 2s
// sampler interval that's ~3 minutes of data.
const MaxMemSamples = 90

// MemSample is one point in the memory history, in megabytes.
type MemSample struct {
	SysMB  float64
	HeapMB float64
}

type APIConfig struct {
	AppName      string
	Platform     string
	Templates    *template.Template
	HomePageHits atomic.Int32
	Guests       *GuestStore

	// High-water marks recorded by the background sampler (bytes).
	PeakSysBytes  atomic.Uint64
	PeakHeapInUse atomic.Uint64

	// samples is a capped ring of recent readings for the live chart. It is
	// filled by the sampler regardless of who's viewing, so chart history is
	// decoupled from request rate.
	samplesMu sync.Mutex
	samples   []MemSample
}

// recordSample appends s to the ring, dropping the oldest once full.
func (c *APIConfig) recordSample(s MemSample) {
	c.samplesMu.Lock()
	defer c.samplesMu.Unlock()
	if len(c.samples) >= MaxMemSamples {
		copy(c.samples, c.samples[1:])
		c.samples[len(c.samples)-1] = s
		return
	}
	c.samples = append(c.samples, s)
}

// Samples returns a copy of the current history, oldest first.
func (c *APIConfig) Samples() []MemSample {
	c.samplesMu.Lock()
	defer c.samplesMu.Unlock()
	out := make([]MemSample, len(c.samples))
	copy(out, c.samples)
	return out
}

// StoreMax atomically raises a to v if v is larger, retrying on contention so
// concurrent writers (the sampler and request handlers) can't lose an update.
func StoreMax(a *atomic.Uint64, v uint64) {
	for {
		cur := a.Load()
		if v <= cur || a.CompareAndSwap(cur, v) {
			return
		}
	}
}

// StartMemorySampler launches a goroutine that periodically reads runtime
// memory stats and records the peak Sys / HeapInuse seen. This catches spikes
// that occur between visits to /api/metrics, which a per-request snapshot alone
// would miss. It runs for the lifetime of the process.
func (c *APIConfig) StartMemorySampler(interval time.Duration) {
	go func() {
		var m runtime.MemStats
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		const mb = 1024.0 * 1024.0
		for {
			runtime.ReadMemStats(&m)
			StoreMax(&c.PeakSysBytes, m.Sys)
			StoreMax(&c.PeakHeapInUse, m.HeapInuse)
			c.recordSample(MemSample{
				SysMB:  float64(m.Sys) / mb,
				HeapMB: float64(m.HeapInuse) / mb,
			})
			<-ticker.C
		}
	}()
}

func (c *APIConfig) MiddlewareCountFirstHomeVisit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("home_seen"); err == http.ErrNoCookie {
			c.HomePageHits.Add(1)

			http.SetCookie(w, &http.Cookie{
				Name:     "home_seen",
				Value:    "1",
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   c.Platform == "prod",
			})
		}

		next.ServeHTTP(w, r)
	})
}
