package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExternalPageCache(url string, ttl time.Duration) *ExternalPageCache {
	return NewExternalPageCache(url, ttl, logrus.New())
}

func TestExternalPageCache_Get(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>Hello</body></html>"))
	}))
	defer upstream.Close()

	cache := newTestExternalPageCache(upstream.URL, 1*time.Hour)

	content, err := cache.Get()
	require.NoError(t, err)
	assert.Equal(t, "<html><body>Hello</body></html>", string(content))
}

func TestExternalPageCache_ReturnsCachedContent(t *testing.T) {
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("<html>response</html>"))
	}))
	defer upstream.Close()

	cache := newTestExternalPageCache(upstream.URL, 1*time.Hour)

	_, err := cache.Get()
	require.NoError(t, err)
	_, err = cache.Get()
	require.NoError(t, err)

	assert.Equal(t, 1, callCount, "should only fetch once while cache is fresh")
}

func TestExternalPageCache_RefreshesAfterTTL(t *testing.T) {
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("<html>response</html>"))
	}))
	defer upstream.Close()

	cache := newTestExternalPageCache(upstream.URL, 1*time.Millisecond)

	_, err := cache.Get()
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, err = cache.Get()
	require.NoError(t, err)

	assert.Equal(t, 2, callCount, "should fetch again after TTL expires")
}

func TestExternalPageCache_ServesStaleOnError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>original</html>"))
	}))

	cache := newTestExternalPageCache(upstream.URL, 1*time.Millisecond)

	content, err := cache.Get()
	require.NoError(t, err)
	assert.Equal(t, "<html>original</html>", string(content))

	// Shut down upstream to simulate failure
	upstream.Close()
	time.Sleep(5 * time.Millisecond)

	// Should return stale content
	content, err = cache.Get()
	require.NoError(t, err)
	assert.Equal(t, "<html>original</html>", string(content))
}

func TestExternalPageCache_ErrorWhenNoCacheAndFetchFails(t *testing.T) {
	cache := newTestExternalPageCache("http://localhost:1", 1*time.Hour)

	_, err := cache.Get()
	assert.Error(t, err)
}

func TestInjectResizeScript_WithBodyTag(t *testing.T) {
	input := []byte("<html><body>content</body></html>")
	result := injectResizeScript(input)

	assert.Contains(t, string(result), "window.dispatchEvent(new Event('resize'))")
	// Script should appear before </body>
	scriptIdx := bytes.Index(result, []byte("<script>"))
	bodyIdx := bytes.Index(result, []byte("</body>"))
	assert.Less(t, scriptIdx, bodyIdx, "resize script should be injected before </body>")
	// Original content and closing tags should still be present
	assert.Contains(t, string(result), "content")
	assert.Contains(t, string(result), "</body></html>")
	// Original input must not be mutated
	assert.Equal(t, "<html><body>content</body></html>", string(input))
}

func TestInjectResizeScript_WithoutBodyTag(t *testing.T) {
	input := []byte("<html><div>content</div></html>")
	result := injectResizeScript(input)

	assert.Contains(t, string(result), "window.dispatchEvent(new Event('resize'))")
	// Script should be appended at the end
	assert.True(t, bytes.HasSuffix(result, resizeScript), "resize script should be appended at end when no </body> tag")
	// Original input must not be mutated
	assert.Equal(t, "<html><div>content</div></html>", string(input))
}

func TestGetExternalPageHTML_FetchError(t *testing.T) {
	// Use an unreachable address so cache.Get() fails with no stale content
	h := &Handlers{
		logger: logrus.New(),
		externalPageCaches: map[string]*ExternalPageCache{
			"broken": newTestExternalPageCache("http://localhost:1", 1*time.Hour),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/external-pages/broken", nil)
	req = mux.SetURLVars(req, map[string]string{"pageSlug": "broken"})
	w := httptest.NewRecorder()

	h.GetExternalPageHTML(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to fetch external page")
}

func TestGetExternalPageHTML_ValidSlug(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>test page</body></html>"))
	}))
	defer upstream.Close()

	h := &Handlers{
		logger: logrus.New(),
		externalPageCaches: map[string]*ExternalPageCache{
			"test-page": newTestExternalPageCache(upstream.URL, 1*time.Hour),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/external-pages/test-page", nil)
	req = mux.SetURLVars(req, map[string]string{"pageSlug": "test-page"})
	w := httptest.NewRecorder()

	h.GetExternalPageHTML(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "test page")
	assert.Contains(t, w.Body.String(), "window.dispatchEvent(new Event('resize'))")
}

func TestGetExternalPageHTML_UnknownSlug(t *testing.T) {
	h := &Handlers{
		logger:             logrus.New(),
		externalPageCaches: map[string]*ExternalPageCache{},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/external-pages/unknown", nil)
	req = mux.SetURLVars(req, map[string]string{"pageSlug": "unknown"})
	w := httptest.NewRecorder()

	h.GetExternalPageHTML(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
