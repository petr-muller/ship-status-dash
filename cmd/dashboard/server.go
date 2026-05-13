package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"ship-status-dash/pkg/auth"
	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

// Server represents the HTTP server for the dashboard API.
type Server struct {
	logger        *logrus.Logger
	configManager *config.Manager[types.DashboardConfig]
	handlers      *Handlers
	corsOrigin    string
	hmacSecret    []byte
	groupCache    *auth.GroupMembershipCache
	httpServer    *http.Server
}

// NewServer creates a new Server instance
func NewServer(configManager *config.Manager[types.DashboardConfig], logger *logrus.Logger, corsOrigin string, hmacSecret []byte, groupCache *auth.GroupMembershipCache, outageManager outage.OutageManager, pingRepo repositories.ComponentPingRepository) *Server {
	return &Server{
		logger:        logger,
		configManager: configManager,
		handlers:      NewHandlers(logger, configManager, outageManager, pingRepo, groupCache),
		corsOrigin:    corsOrigin,
		hmacSecret:    hmacSecret,
		groupCache:    groupCache,
	}
}

type route struct {
	path      string
	method    string
	handler   func(http.ResponseWriter, *http.Request)
	protected bool
}

func (s *Server) setupRoutes() http.Handler {
	routes := []route{
		{
			path:      "/health",
			method:    http.MethodGet,
			handler:   s.handlers.HealthJSON,
			protected: false,
		},
		{
			path:      "/api/status",
			method:    http.MethodGet,
			handler:   s.handlers.GetAllComponentsStatusJSON,
			protected: false,
		},
		{
			path:      "/api/status/{componentName}",
			method:    http.MethodGet,
			handler:   s.handlers.GetComponentStatusJSON,
			protected: false,
		},
		{
			path:      "/api/status/{componentName}/{subComponentName}",
			method:    http.MethodGet,
			handler:   s.handlers.GetSubComponentStatusJSON,
			protected: false,
		},
		{
			path:      "/api/components",
			method:    http.MethodGet,
			handler:   s.handlers.GetComponentsJSON,
			protected: false,
		},
		{
			path:      "/api/tags",
			method:    http.MethodGet,
			handler:   s.handlers.ListTagsJSON,
			protected: false,
		},
		{
			path:      "/api/sub-components",
			method:    http.MethodGet,
			handler:   s.handlers.ListSubComponentsJSON,
			protected: false,
		},
		{
			path:      "/api/outages/during",
			method:    http.MethodGet,
			handler:   s.handlers.GetOutagesDuringJSON,
			protected: false,
		},
		{
			path:      "/api/components/{componentName}",
			method:    http.MethodGet,
			handler:   s.handlers.GetComponentInfoJSON,
			protected: false,
		},
		{
			path:      "/api/components/{componentName}/outages",
			method:    http.MethodGet,
			handler:   s.handlers.GetOutagesJSON,
			protected: false,
		},
		{
			path:      "/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}",
			method:    http.MethodGet,
			handler:   s.handlers.GetOutageJSON,
			protected: false,
		},
		{
			path:      "/api/components/{componentName}/{subComponentName}/outages",
			method:    http.MethodGet,
			handler:   s.handlers.GetSubComponentOutagesJSON,
			protected: false,
		},
		{
			path:      "/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}/audit-logs",
			method:    http.MethodGet,
			handler:   s.handlers.GetOutageAuditLogsJSON,
			protected: false,
		},
		{
			path:      "/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}",
			method:    http.MethodPatch,
			handler:   s.handlers.UpdateOutageJSON,
			protected: true,
		},
		{
			path:      "/api/components/{componentName}/{subComponentName}/outages/{outageId:[0-9]+}",
			method:    http.MethodDelete,
			handler:   s.handlers.DeleteOutage,
			protected: true,
		},
		{
			path:      "/api/components/{componentName}/{subComponentName}/outages",
			method:    http.MethodPost,
			handler:   s.handlers.CreateOutageJSON,
			protected: true,
		},
		{
			path:      "/api/user",
			method:    http.MethodGet,
			handler:   s.handlers.GetAuthenticatedUserJSON,
			protected: true,
		},
		{
			path:      "/api/component-monitor/report",
			method:    http.MethodPost,
			handler:   s.handlers.PostComponentMonitorReportJSON,
			protected: true,
		},
		{
			path:      "/api/external-pages/{pageSlug}",
			method:    http.MethodGet,
			handler:   s.handlers.GetExternalPageHTML,
			protected: false,
		},
	}

	router := mux.NewRouter()
	protectedRouter := router.Name("protected").Subrouter()
	protectedRouter.Use(func(next http.Handler) http.Handler {
		return newAuthMiddleware(s.logger, s.hmacSecret, next)
	})

	for _, route := range routes {
		if route.protected {
			protectedRouter.HandleFunc(route.path, route.handler).Methods(route.method)
		} else {
			router.HandleFunc(route.path, route.handler).Methods(route.method)
		}
	}

	// Serve static files (React frontend) - must be after API routes
	spa := spaHandler{staticPath: "./static", indexPath: "index.html"}
	router.PathPrefix("/").Handler(spa)

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{s.corsOrigin}),
		handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization", "X-Forwarded-User", "GAP-Signature"}),
		handlers.AllowCredentials(),
	)(router)

	handler := s.loggingMiddleware(corsHandler)

	return handler
}

// spaHandler implements the http.Handler interface for serving a Single Page Application.
// It serves static files if they exist, otherwise serves index.html to allow
// client-side routing to work.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.staticPath, r.URL.Path)

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}

	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		s.logger.WithFields(logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"duration": time.Since(start),
		}).Info("Request processed")
	})
}

// Start begins listening for HTTP requests on the specified address.
func (s *Server) Start(addr string) error {
	handler := s.setupRoutes()
	s.logger.Infof("Starting dashboard server on %s", addr)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.logger.Info("Shutting down dashboard server")
	return s.httpServer.Shutdown(ctx)
}
