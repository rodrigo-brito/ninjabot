package ui

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
)

//go:embed templates/*.html
var templateFiles embed.FS

var (
	indexTemplate = template.Must(template.ParseFS(templateFiles, "templates/index.html"))
	errorTemplate = template.Must(template.ParseFS(templateFiles, "templates/error.html"))
)

// router wires the API, the bundle static files and the index page.
func (c *Chart) router(b bundle, bundleErr error) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", c.handleHealth)
		r.Get("/pairs", c.handlePairs)
		r.Get("/events", c.handleEvents)
		r.Get("/controls", c.handleControls)
		r.Post("/controls/start", c.handleControlStart)
		r.Post("/controls/stop", c.handleControlStop)
		r.Post("/controls/order", c.handleControlOrder)
		r.Get("/{pair}/snapshot", c.handleSnapshot)
		r.Get("/{pair}/orders.csv", c.handleOrdersCSV)
	})

	if bundleErr == nil {
		fs := http.FileServer(http.Dir(b.dir))
		r.Handle("/ui/{version}/*", http.StripPrefix("/ui/"+b.version, bundleCacheHeaders(b, fs)))
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := indexTemplate.Execute(w, map[string]string{"Version": b.version}); err != nil {
				log.Error(err)
			}
		})
	} else {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			if err := errorTemplate.Execute(w, map[string]string{
				"Error":      bundleErr.Error(),
				"EnvDir":     envUIDir,
				"EnvVersion": envUIVersion,
			}); err != nil {
				log.Error(err)
			}
		})
	}

	return r
}

// bundleCacheHeaders makes versioned assets immutable and local (dev)
// assets uncached.
func bundleCacheHeaders(b bundle, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b.version == versionLocal {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error(err)
	}
}

func (c *Chart) handleHealth(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	lastUpdate := c.lastUpdate
	c.mu.Unlock()

	if time.Since(lastUpdate) > time.Hour+10*time.Minute {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(lastUpdate.String()))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Chart) handlePairs(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	pairs := c.pairs()
	c.mu.Unlock()
	writeJSON(w, PairsResponse{Pairs: pairs})
}

func (c *Chart) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	pair := strings.ToUpper(chi.URLParam(r, "pair"))

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.candles[pair]; !ok {
		http.Error(w, "unknown pair", http.StatusNotFound)
		return
	}
	writeJSON(w, c.snapshot(pair))
}

func (c *Chart) handleOrdersCSV(w http.ResponseWriter, r *http.Request) {
	pair := strings.ToUpper(chi.URLParam(r, "pair"))

	c.mu.Lock()
	_, ok := c.candles[pair]
	rows := c.orderRowsByPair(pair)
	c.mu.Unlock()

	if !ok {
		http.Error(w, "unknown pair", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=history_"+pair+".csv")

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"created_at", "status", "side", "id", "type", "quantity", "price", "total", "profit"})
	_ = writer.WriteAll(rows)
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Errorf("ui: failed writing csv: %v", err)
	}
}
