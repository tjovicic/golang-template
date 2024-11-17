package main

import (
	"context"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	internalApi "github.com/tjovicic/golang-template/internal/api"
	internalHttp "github.com/tjovicic/golang-template/internal/http"
	internalLogger "github.com/tjovicic/golang-template/internal/logger"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type config struct {
	ServiceName     string        `required:"true" split_words:"true"`
	Environment     string        `required:"true"`
	LogLevel        string        `default:"info" split_words:"true"`
	StartupTimeout  time.Duration `default:"15s" split_words:"true"`
	ShutdownTimeout time.Duration `default:"15s" split_words:"true"`
}

func main() {
	var env config

	if err := envconfig.Process("", &env); err != nil {
		panic(fmt.Sprintf("reading env variables: %v", err))
	}

	logger, err := internalLogger.NewLogger(env.ServiceName, env.LogLevel)
	if err != nil {
		panic(fmt.Sprintf("creating a logger: %v", err))
	}

	ctx, cancel := startupContext(env, logger)
	defer cancel()

	s, err := internalHttp.NewServer(ctx)
	if err != nil {
		logger.Err(err).Msgf("creating a server")
		return
	}

	h, err := internalHttp.NewHandler(ctx)
	if err != nil {
		logger.Err(err).Msg("creating a handler")
		return
	}

	s.Router().HandleFunc("/handle", internalApi.GetHandler(h)).Methods(http.MethodGet)

	gracefulShutdown(env, logger, s, h)
	if err = s.ListenAndServe(ctx); err != nil {
		logger.Err(err).Msg("starting server")
	}
}

func startupContext(env config, logger zerolog.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), env.StartupTimeout)
	ctx = logger.WithContext(ctx)
	return ctx, cancel
}

func shutdownContext(env config) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), env.ShutdownTimeout)
	return ctx, cancel
}

func gracefulShutdown(env config, logger zerolog.Logger, s *internalHttp.Server, h *internalHttp.Handler) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func(logger zerolog.Logger) {
		<-c
		logger.Info().Msg("received shutdown signal")

		shutdownCtx, cancel := shutdownContext(env)
		defer cancel()

		h.Close()
		s.Close(shutdownCtx)

		close(c)
	}(logger)
}
