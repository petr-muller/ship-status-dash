package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// resizeScript is injected into proxied HTML to trigger a window resize event
// after loading. This ensures embedded chart libraries (Plotly, Chart.js, etc.)
// recalculate their layout to fit the iframe dimensions.
var resizeScript = []byte(`<script>window.addEventListener('load',function(){setTimeout(function(){window.dispatchEvent(new Event('resize'))},100)})</script>`)

// ExternalPageCache fetches and caches HTML content from an external URL.
type ExternalPageCache struct {
	mu        sync.RWMutex
	content   []byte
	fetchedAt time.Time
	ttl       time.Duration
	sourceURL string
	logger    *logrus.Logger
	client    *http.Client
}

// NewExternalPageCache creates a new cache for the given URL with the specified TTL.
func NewExternalPageCache(sourceURL string, ttl time.Duration, logger *logrus.Logger) *ExternalPageCache {
	return &ExternalPageCache{
		sourceURL: sourceURL,
		ttl:       ttl,
		logger:    logger,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Get returns the cached content, fetching from the source if the cache is stale or empty.
func (c *ExternalPageCache) Get() ([]byte, error) {
	c.mu.RLock()
	if len(c.content) > 0 && time.Since(c.fetchedAt) < c.ttl {
		content := c.content
		c.mu.RUnlock()
		return content, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if len(c.content) > 0 && time.Since(c.fetchedAt) < c.ttl {
		return c.content, nil
	}

	content, err := c.fetch()
	if err != nil {
		if len(c.content) > 0 {
			c.logger.WithError(err).Warn("Failed to refresh external page, serving stale content")
			return c.content, nil
		}
		return nil, err
	}

	c.content = content
	c.fetchedAt = time.Now()
	return c.content, nil
}

func (c *ExternalPageCache) fetch() ([]byte, error) {
	u, err := url.Parse(c.sourceURL)
	if err != nil {
		return nil, fmt.Errorf("parsing source URL: %w", err)
	}
	q := u.Query()
	q.Set("_t", strconv.FormatInt(time.Now().Unix(), 10))
	u.RawQuery = q.Encode()
	resp, err := c.client.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("fetching external page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading external page body: %w", err)
	}

	return body, nil
}

// GetExternalPageHTML serves cached HTML content for the requested external page.
func (h *Handlers) GetExternalPageHTML(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pageSlug := vars["pageSlug"]

	cache, exists := h.externalPageCaches[pageSlug]
	if !exists {
		respondWithError(w, http.StatusNotFound, "External page not found")
		return
	}

	content, err := cache.Get()
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch external page")
		respondWithError(w, http.StatusBadGateway, "Failed to fetch external page")
		return
	}

	// Clone content before modifying to avoid mutating the cached slice
	buf := make([]byte, len(content))
	copy(buf, content)

	// Inject resize script to help embedded charts render at the correct size
	if idx := bytes.LastIndex(buf, []byte("</body>")); idx != -1 {
		buf = append(buf[:idx], append(resizeScript, buf[idx:]...)...)
	} else {
		buf = append(buf, resizeScript...)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(buf)
}
