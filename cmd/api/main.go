package main

import (
	"context"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	internalHttp "github.com/tjovicic/golang-template/internal/http"
	internalLogger "github.com/tjovicic/golang-template/internal/logger"
	"net/http"
	"time"
)

type config struct {
	ServiceName     string        `required:"true" split_words:"true"`
	Environment     string        `required:"true"`
	LogLevel        string        `default:"info" split_words:"true"`
	StartupTimeout  time.Duration `default:"15s" split_words:"true"`
	ShutdownTimeout time.Duration `default:"15s" split_words:"true"`
	ProfilerPort    string        `default:"6060" split_words:"true"`
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

	s.Router().HandleFunc("/handle", GetHandler(h)).Methods(http.MethodGet)

	//gracefulShutdown(ctx, env, s, h)

	if err = s.ListenAndServe(ctx); err != nil {
		logger.Err(err).Msg("starting server")
	}
}

func startupContext(env config, logger zerolog.Logger) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), env.StartupTimeout)
	ctx = logger.WithContext(ctx)
	return ctx, cancel
}
