package http

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	internalContext "github.com/tjovicic/golang-template/internal/context"
	"github.com/tjovicic/golang-template/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"net/http"
	"time"
)

type Server struct {
	config        serverConfig
	srv           *http.Server
	isHealthy     bool
	router        *mux.Router
	traceCleanup  func(context.Context)
	metricCleanup func(context.Context)
}

type serverConfig struct {
	ServiceName    string `required:"true" split_words:"true"`
	ServiceVersion string `default:"v1.0.0"`
	Port           string `required:"true"`
	Environment    string `required:"true"`
	LogLevel       string `default:"info" split_words:"true"`

	HandlerTimeout    int `default:"15" split_words:"true"`
	WriteTimeout      int `default:"10" split_words:"true"`
	ReadTimeout       int `default:"5" split_words:"true"`
	ReadHeaderTimeout int `default:"5" split_words:"true"`
	IdleTimeout       int `default:"15" split_words:"true"`

	CollectorURL string `required:"true" split_words:"true"`
}

func NewServer(ctx context.Context) (*Server, error) {
	var env serverConfig

	if err := envconfig.Process("", &env); err != nil {
		return nil, fmt.Errorf("failed to read env variables: %v", err)
	}

	traceCleanup, err := telemetry.InstallTraceExportPipeline(ctx, telemetry.TraceConfig{
		Config: telemetry.Config{
			Name:         env.ServiceName,
			Version:      env.ServiceVersion,
			Environment:  env.Environment,
			CollectorURL: env.CollectorURL,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("installing trace provider: %v", err)
	}

	metricCleanup, err := telemetry.InstallMetricExportPipeline(ctx, telemetry.MeterConfig{
		Config: telemetry.Config{
			Name:         env.ServiceName,
			Version:      env.ServiceVersion,
			Environment:  env.Environment,
			CollectorURL: env.CollectorURL,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("installing metric provider: %v", err)
	}

	router := mux.NewRouter()
	router.Use(otelmux.Middleware(env.ServiceName))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			childCtx := internalContext.WithID(r.Context())
			childCtx = log.Ctx(ctx).WithContext(childCtx)
			next.ServeHTTP(w, r.WithContext(childCtx))
		})
	})

	// In case you need a performance boost, consider https://github.com/valyala/fasthttp
	srv := &http.Server{
		Handler:           http.TimeoutHandler(router, time.Duration(env.HandlerTimeout)*time.Second, "Server timed out"),
		Addr:              ":" + env.Port,
		WriteTimeout:      time.Duration(env.WriteTimeout) * time.Second,
		ReadTimeout:       time.Duration(env.ReadTimeout) * time.Second,
		IdleTimeout:       time.Duration(env.IdleTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(env.ReadHeaderTimeout) * time.Second,
	}

	return &Server{
		config:        env,
		router:        router,
		srv:           srv,
		isHealthy:     true,
		traceCleanup:  traceCleanup,
		metricCleanup: metricCleanup,
	}, nil
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	s.router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !s.isHealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, "healthy")
	}).Methods(http.MethodGet)

	log.Ctx(ctx).Info().Msgf("running on port %v", s.config.Port)
	return s.srv.ListenAndServe()
}

func (s *Server) Router() *mux.Router {
	return s.router
}

func (s *Server) Close(ctx context.Context) {
	s.isHealthy = false

	s.traceCleanup(ctx)
	s.metricCleanup(ctx)

	if err := s.srv.Shutdown(ctx); err != nil {
		log.Ctx(ctx).Err(err).Msg("closing server")
	}
}
