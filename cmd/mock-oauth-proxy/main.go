package main

import (
	"context"
	"crypto"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ship-status-dash/pkg/auth"

	"github.com/18F/hmacauth"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

type User struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
	Email        string `yaml:"email"`
}

type ServiceAccount struct {
	Name  string `yaml:"name"`
	Token string `yaml:"token"`
}

type Config struct {
	Users           []User           `yaml:"users"`
	ServiceAccounts []ServiceAccount `yaml:"service_accounts"`
}

func authenticateServiceAccount(token string, config *Config) (*ServiceAccount, error) {
	for _, sa := range config.ServiceAccounts {
		if sa.Token == token {
			return &sa, nil
		}
	}
	return nil, fmt.Errorf("invalid service account token")
}

type Options struct {
	ConfigPath     string
	Port           string
	Upstream       string
	HMACSecretFile string
	FrontendDevURL string
}

func NewOptions() *Options {
	opts := &Options{}
	flag.StringVar(&opts.ConfigPath, "config", "", "Path to config file")
	flag.StringVar(&opts.Port, "port", "8443", "Port to listen on")
	flag.StringVar(&opts.Upstream, "upstream", "", "Upstream server URL")
	flag.StringVar(&opts.HMACSecretFile, "hmac-secret-file", "", "File containing HMAC secret")
	flag.StringVar(
		&opts.FrontendDevURL,
		"frontend-dev-url",
		"http://localhost:3030",
		"OAuth callback redirect and default CORS origin for local Vite (include scheme, no trailing slash)",
	)
	flag.Parse()
	return opts
}

func (o *Options) Validate() error {
	if o.ConfigPath == "" {
		return errors.New("config path is required (use --config flag)")
	}
	if _, err := os.Stat(o.ConfigPath); os.IsNotExist(err) {
		return errors.New("config file does not exist: " + o.ConfigPath)
	}
	if o.Upstream == "" {
		return errors.New("upstream URL is required (use --upstream flag)")
	}
	if o.HMACSecretFile == "" {
		return errors.New("hmac secret file is required (use --hmac-secret-file flag)")
	}
	if _, err := os.Stat(o.HMACSecretFile); os.IsNotExist(err) {
		return errors.New("hmac secret file does not exist: " + o.HMACSecretFile)
	}
	return nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func getHMACSecret(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read HMAC secret file: %w", err)
	}
	return []byte(strings.TrimSpace(string(data))), nil
}

func authenticateUser(username, password string, config *Config) (*User, error) {
	for _, user := range config.Users {
		if user.Username == username {
			err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
			if err != nil {
				return nil, fmt.Errorf("invalid password")
			}
			return &user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func oauthStartHandler(config *Config, logger *logrus.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := authenticateUser(username, password, config)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"username": username,
				"error":    err,
			}).Warn("Authentication failed")
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		logger.WithFields(logrus.Fields{
			"username": user.Username,
		}).Info("User authenticated, redirecting to callback")

		http.Redirect(w, r, "/oauth/callback", http.StatusFound)
	})
}

func oauthCallbackHandler(frontendDevURL string) http.Handler {
	redirect := strings.TrimSuffix(frontendDevURL, "/") + "/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirect, http.StatusFound)
	})
}

func basicAuthHandler(
	config *Config,
	upstreamURL *url.URL,
	hmacAuth hmacauth.HmacAuth,
	logger *logrus.Logger,
	frontendDevURL string,
) http.Handler {
	defaultOrigin := strings.TrimSuffix(frontendDevURL, "/")
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		if req.Header.Get("Date") == "" {
			req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
		}

		if req.ContentLength > 0 {
			req.Header.Set("Content-Length", fmt.Sprintf("%d", req.ContentLength))
		}

		hmacAuth.SignRequest(req)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLogger := logger.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"query":  r.URL.RawQuery,
		})

		requestLogger.Info("Incoming request")

		// Allow OPTIONS preflight requests to pass through without authentication
		if r.Method == http.MethodOptions {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = defaultOrigin
			}
			requestLogger.WithField("origin", origin).Info("Handling OPTIONS preflight request")
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Forwarded-User, GAP-Signature")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var forwardedUser string
		var forwardedEmail string

		// Authenticate request
		authHeader := r.Header.Get("Authorization")
		if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
			sa, err := authenticateServiceAccount(token, config)
			if err != nil {
				requestLogger.WithField("error", err).Warn("Service account authentication failed")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			forwardedUser = sa.Name
			forwardedEmail = ""
			requestLogger.WithFields(logrus.Fields{
				"auth_type":       "bearer",
				"service_account": sa.Name,
			}).Info("Service account authenticated")
		} else {
			username, password, ok := r.BasicAuth()
			if !ok {
				requestLogger.Warn("No BasicAuth credentials provided")
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := authenticateUser(username, password, config)
			if err != nil {
				requestLogger.WithFields(logrus.Fields{
					"username": username,
					"error":    err,
				}).Warn("Authentication failed")
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			forwardedUser = user.Username
			forwardedEmail = user.Email
			requestLogger.WithFields(logrus.Fields{
				"username":  user.Username,
				"auth_type": "basic",
			}).Info("User authenticated")
		}

		// Set forwarded headers for upstream
		r.Header.Set("X-Forwarded-User", forwardedUser)
		if forwardedEmail != "" {
			r.Header.Set("X-Forwarded-Email", forwardedEmail)
		}
		r.Header.Set("X-Forwarded-Access-Token", "mock-access-token-"+forwardedUser)

		responseLogger := &responseLoggingWriter{
			ResponseWriter: w,
			logger:         requestLogger,
		}

		proxy.ServeHTTP(responseLogger, r)
	})
}

// responseLoggingWriter wraps http.ResponseWriter to log response details
type responseLoggingWriter struct {
	http.ResponseWriter
	logger     *logrus.Entry
	statusCode int
}

func (w *responseLoggingWriter) WriteHeader(code int) {
	w.statusCode = code
	w.logger.WithFields(logrus.Fields{
		"status_code": code,
	}).Info("Upstream response received")
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseLoggingWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	w.logger.WithFields(logrus.Fields{
		"status_code": w.statusCode,
		"body_length": len(b),
	}).Info("Response body written")
	return w.ResponseWriter.Write(b)
}

func setupLogger() *logrus.Logger {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return log
}

func main() {
	logger := setupLogger()
	opts := NewOptions()

	if err := opts.Validate(); err != nil {
		logger.WithField("error", err).Fatal("Invalid options")
	}

	config, err := loadConfig(opts.ConfigPath)
	if err != nil {
		logger.WithField("error", err).Fatal("Failed to load config")
	}

	hmacSecret, err := getHMACSecret(opts.HMACSecretFile)
	if err != nil {
		logger.WithField("error", err).Fatal("Failed to load HMAC secret")
	}

	upstreamURL, err := url.Parse(opts.Upstream)
	if err != nil {
		logger.WithField("error", err).Fatal("Failed to parse upstream URL")
	}

	hmacAuth := hmacauth.NewHmacAuth(crypto.SHA256, hmacSecret, auth.GAPSignatureHeader, auth.OAuthSignatureHeaders)

	unauthProxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	router := mux.NewRouter()
	router.Handle("/health", unauthProxy)
	router.Handle("/oauth/start", oauthStartHandler(config, logger))
	router.Handle("/oauth/callback", oauthCallbackHandler(opts.FrontendDevURL))
	router.PathPrefix("/").Handler(basicAuthHandler(config, upstreamURL, hmacAuth, logger, opts.FrontendDevURL))

	server := &http.Server{
		Addr:              ":" + opts.Port,
		Handler:           router,
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithFields(logrus.Fields{
				"port":  opts.Port,
				"error": err,
			}).Fatal("Server failed to start")
		}
	}()

	logger.WithFields(logrus.Fields{
		"port":     opts.Port,
		"upstream": opts.Upstream,
	}).Info("Mock oauth-proxy started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.WithField("error", err).Error("Server shutdown failed")
	}
}
