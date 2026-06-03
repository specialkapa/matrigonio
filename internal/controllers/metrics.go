package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"strings"

	"github.com/starfederation/datastar-go/datastar"
	"github.com/specialkapa/matrigonio/internal/server"
)

// freeTierLimitMB is the Render free-tier RAM ceiling we chart the peak against.
const freeTierLimitMB = 512.0

type MetricsController struct {
	*server.APIConfig
}

// bytesToMB converts a byte count to megabytes, rounded to one decimal place.
func bytesToMB(b uint64) float64 {
	return float64(b) / 1024.0 / 1024.0
}

func (c *MetricsController) HandlerMetrics(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Fold this live reading into the high-water marks too, so a spike landing
	// exactly on a request (between sampler ticks) is never lost.
	server.StoreMax(&c.PeakSysBytes, m.Sys)
	server.StoreMax(&c.PeakHeapInUse, m.HeapInuse)

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(struct {
		Hits int32 `json:"hits"`
		// HeapInUseMB is live heap currently in use by the app.
		HeapInUseMB float64 `json:"heap_in_use_mb"`
		// SysMB is total memory reserved from the OS — the best proxy for the
		// figure a host (Render) counts against the instance's RAM limit.
		SysMB float64 `json:"sys_mb"`
		// Peak* are the largest values seen since startup (the worst case).
		PeakSysMB       float64 `json:"peak_sys_mb"`
		PeakHeapInUseMB float64 `json:"peak_heap_in_use_mb"`
		NumGoroutine    int     `json:"num_goroutine"`
		NumGC           uint32  `json:"num_gc"`
	}{
		Hits:            (*c).HomePageHits.Load(),
		HeapInUseMB:     bytesToMB(m.HeapInuse),
		SysMB:           bytesToMB(m.Sys),
		PeakSysMB:       bytesToMB(c.PeakSysBytes.Load()),
		PeakHeapInUseMB: bytesToMB(c.PeakHeapInUse.Load()),
		NumGoroutine:    runtime.NumGoroutine(),
		NumGC:           m.NumGC,
	})
	_, _ = w.Write(data)
}

// HandlerMetricsFragment renders the live dashboard as an HTML fragment and
// streams it as a Datastar morph patch. The page triggers this on a 2s interval
// (data-on-interval) and Datastar morphs the result into #metrics — so all the
// rendering lives here on the server and the page carries no custom JS.
func (c *MetricsController) HandlerMetricsFragment(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	server.StoreMax(&c.PeakSysBytes, m.Sys)
	server.StoreMax(&c.PeakHeapInUse, m.HeapInuse)

	v := metricsView{
		Hits:       c.HomePageHits.Load(),
		SysMB:      bytesToMB(m.Sys),
		HeapMB:     bytesToMB(m.HeapInuse),
		PeakSysMB:  bytesToMB(c.PeakSysBytes.Load()),
		PeakHeapMB: bytesToMB(c.PeakHeapInUse.Load()),
		Goroutines: runtime.NumGoroutine(),
		GC:         m.NumGC,
		Samples:    c.Samples(),
	}
	// Default patch mode (outer) morphs by id. No view transition: the fragment
	// refreshes every 2s and a cross-fade on each tick would flicker, whereas an
	// in-place morph updates only what changed.
	sse := datastar.NewSSE(w, r)
	_ = sse.PatchElements(renderMetricsFragment(v))
}

type metricsView struct {
	Hits                  int32
	SysMB, HeapMB         float64
	PeakSysMB, PeakHeapMB float64
	Goroutines            int
	GC                    uint32
	Samples               []server.MemSample
}

func renderMetricsFragment(v metricsView) string {
	var b strings.Builder
	b.WriteString(`<div id="metrics">`)

	// --- number cards ---
	b.WriteString(`<section class="cards">`)
	card(&b, "accent-sys", "Sys (live)", num(v.SysMB), "MB", "reserved from OS")
	card(&b, "accent-peak", "Sys (peak)", num(v.PeakSysMB), "MB", "worst since startup")
	card(&b, "accent-heap", "Heap in use", num(v.HeapMB), "MB", "peak "+num(v.PeakHeapMB)+" MB")
	card(&b, "", "Goroutines", fmt.Sprintf("%d", v.Goroutines), "", "")
	card(&b, "", "Home hits", fmt.Sprintf("%d", v.Hits), "", "")
	card(&b, "", "GC cycles", fmt.Sprintf("%d", v.GC), "", "")
	b.WriteString(`</section>`)

	// --- chart ---
	b.WriteString(`<section class="panel"><h2>Memory over time</h2>`)
	b.WriteString(renderSparkline(v.Samples, v.PeakSysMB))
	b.WriteString(`<div class="legend"><span class="l-sys">sys (live)</span>` +
		`<span class="l-heap">heap in use</span><span class="l-peak">sys peak</span></div>`)
	b.WriteString(`</section>`)

	// --- peak vs limit bar ---
	b.WriteString(`<section class="panel"><h2>Peak vs free-tier limit (512 MB)</h2>`)
	renderBar(&b, v.PeakSysMB)
	b.WriteString(`</section>`)

	b.WriteString(`</div>`)
	return b.String()
}

// num formats a megabyte figure to two decimals.
func num(v float64) string { return fmt.Sprintf("%.2f", v) }

func card(b *strings.Builder, accent, label, value, unit, sub string) {
	cls := "card"
	if accent != "" {
		cls += " " + accent
	}
	fmt.Fprintf(b, `<div class="%s"><div class="label">%s</div><div class="value">%s`, cls, label, value)
	if unit != "" {
		fmt.Fprintf(b, `<span class="unit">%s</span>`, unit)
	}
	b.WriteString(`</div>`)
	if sub != "" {
		fmt.Fprintf(b, `<div class="sub">%s</div>`, sub)
	}
	b.WriteString(`</div>`)
}

func renderBar(b *strings.Builder, peak float64) {
	pct := peak / freeTierLimitMB * 100
	if pct > 100 {
		pct = 100
	}
	width := pct
	if width < 0.8 {
		width = 0.8
	}
	color := "var(--ok)"
	switch {
	case pct >= 85:
		color = "var(--danger)"
	case pct >= 60:
		color = "var(--warn)"
	}
	fmt.Fprintf(b, `<div class="bar-wrap"><div class="bar-fill" style="width:%.2f%%;background:%s"></div>`+
		`<div class="bar-text">%s MB / %.0f MB (%.2f%%)</div></div>`,
		width, color, num(peak), freeTierLimitMB, pct)
	fmt.Fprintf(b, `<div class="bar-caption">Headroom: %s MB before the free-tier limit.</div>`,
		num(freeTierLimitMB-peak))
}

// renderSparkline draws the sys/heap history as an inline SVG. It's regenerated
// server-side each tick and morphed in, so no client-side charting code is
// needed. viewBox keeps it responsive to the container width.
func renderSparkline(samples []server.MemSample, peakSys float64) string {
	const w, h, pad, axis = 880.0, 220.0, 28.0, 30.0
	x0, x1 := pad+axis, w-8
	y0, y1 := pad, h-pad

	// Scale to the largest value in view (incl. the peak line), with headroom.
	max := peakSys
	for _, s := range samples {
		max = math.Max(max, math.Max(s.SysMB, s.HeapMB))
	}
	max = math.Max(max*1.2, 4)

	xAt := func(i int) float64 {
		if len(samples) <= 1 {
			return x1
		}
		return x0 + (x1-x0)*float64(i)/float64(len(samples)-1)
	}
	yAt := func(val float64) float64 { return y0 + (y1-y0)*(1-val/max) }

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="chart" viewBox="0 0 %.0f %.0f" preserveAspectRatio="none" role="img" aria-label="memory over time">`, w, h)

	// gridlines + y labels
	for g := 0; g <= 4; g++ {
		y := y0 + (y1-y0)*float64(g)/4
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#1f2633" stroke-width="1"/>`, x0, y, x1, y)
		fmt.Fprintf(&b, `<text x="2" y="%.1f" fill="#7e8aa0" font-size="11" font-family="monospace">%s</text>`, y+3, num(max*(1-float64(g)/4)))
	}

	// dashed peak line
	py := yAt(peakSys)
	fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#ff8a5c" stroke-width="1" stroke-dasharray="5 4"/>`, x0, py, x1, py)

	// data polylines
	b.WriteString(polyline(samples, xAt, yAt, func(s server.MemSample) float64 { return s.SysMB }, "#5ad1c8"))
	b.WriteString(polyline(samples, xAt, yAt, func(s server.MemSample) float64 { return s.HeapMB }, "#c79bff"))

	b.WriteString(`</svg>`)
	return b.String()
}

func polyline(samples []server.MemSample, xAt func(int) float64, yAt func(float64) float64, get func(server.MemSample) float64, color string) string {
	if len(samples) == 0 {
		return ""
	}
	var pts strings.Builder
	for i, s := range samples {
		if i > 0 {
			pts.WriteByte(' ')
		}
		fmt.Fprintf(&pts, "%.1f,%.1f", xAt(i), yAt(get(s)))
	}
	return fmt.Sprintf(`<polyline fill="none" stroke="%s" stroke-width="2" points="%s"/>`, color, pts.String())
}

// TODO: figure out how to prevent anyone from hitting this endpoint without requiring auth
func (c *MetricsController) HandlerReset(w http.ResponseWriter, r *http.Request) {
	if c.Platform != "dev" {
		responseWithError(w, errors.New("unauthorized access to reset endpoint"), "Unauthorized", 403)
		return
	}
	(*c).HomePageHits.Store(0)

	w.WriteHeader(200)
	_, _ = w.Write([]byte("Hits reset to 0 and users purged"))
}

func (c *MetricsController) HandlerResetHomeCookie(w http.ResponseWriter, r *http.Request) {
	if c.Platform != "dev" {
		responseWithError(w, errors.New("unauthorized access to reset endpoint"), "Unauthorized", 403)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "home_seen",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Platform == "prod",
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("home_seen cookie cleared"))
}
