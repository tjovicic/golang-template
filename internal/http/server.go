package http

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	internalContext "github.com/tjovicic/golang-template/internal/context"
	"github.com/tjovicic/golang-template/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/metrics"
	"syscall"
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

	ReadTimeout       int `default:"5" split_words:"true"`
	ReadHeaderTimeout int `default:"5" split_words:"true"`
	// WriteTimeout should be changed in case you are doing a cpu profile
	WriteTimeout int `default:"10" split_words:"true"`
	// HandlerTimeout should be changed in case you are doing a cpu profile
	HandlerTimeout int `default:"15" split_words:"true"`
	IdleTimeout    int `default:"15" split_words:"true"`

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

	// Add the pprof routes
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	router.Handle("/debug/pprof/block", pprof.Handler("block"))
	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			childCtx := internalContext.WithID(r.Context())
			childCtx = log.Ctx(ctx).WithContext(childCtx)
			next.ServeHTTP(w, r.WithContext(childCtx))
		})
	})

	// In case you need a performance boost, consider https://github.com/valyala/fasthttp
	httpServer := &http.Server{
		Handler:           http.TimeoutHandler(router, time.Duration(env.HandlerTimeout)*time.Second, "Server timed out"),
		Addr:              ":" + env.Port,
		WriteTimeout:      time.Duration(env.WriteTimeout) * time.Second,
		ReadTimeout:       time.Duration(env.ReadTimeout) * time.Second,
		IdleTimeout:       time.Duration(env.IdleTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(env.ReadHeaderTimeout) * time.Second,
	}

	server := &Server{
		config:        env,
		router:        router,
		srv:           httpServer,
		isHealthy:     true,
		traceCleanup:  traceCleanup,
		metricCleanup: metricCleanup,
	}

	gracefulServerShutdown(ctx, server)

	return server, nil
}

func startAutoProfiler(ctx context.Context) error {
	memSamples := make([]metrics.Sample, 2)
	memSamples[0].Name = "/memory/classes/total:bytes"
	memSamples[1].Name = "/memory/classes/heap/released:bytes"

	profileLimit := cgroupmemlimited.LimitAfterInit * (*autoProfilerPercentageThreshold / 100)

	go func() {
		ticker := time.NewTicker(*profilerInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics.Read(memSamples)
				inUseMem := memSamples[0].Value.Uint64() - memSamples[1].Value.Uint64()

				if inUseMem > profileLimit {
					if !limiter.Allow() { // rate limiter
						continue
					}

					var buf bytes.Buffer
					if err := pprof.WriteHeapProfile(&buf); err != nil {
						continue
					}

					// Upload the profile to object storage
				}
			}
		}
	}()
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

func gracefulServerShutdown(ctx context.Context, server *Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c
		log.Ctx(ctx).Info().Msg("received server shutdown signal")

		server.Close(ctx)

		close(c)
	}()
}
