package http

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"github.com/tjovicic/golang-template/internal/postgres"
	"go.opentelemetry.io/otel/metric"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Handler struct {
	config   handlerConfig
	dbClient *pgxpool.Pool
	meter    metric.Meter
}

type handlerConfig struct {
	PGUser        string `required:"true" split_words:"true"`
	PGPassword    string `required:"true" split_words:"true"`
	PGHost        string `required:"true" split_words:"true"`
	PGName        string `required:"true" split_words:"true"`
	PGPort        string `default:"5432" split_words:"true"`
	PGSslRootCert string `split_words:"true"`
	PGSslKey      string `split_words:"true"`
	PGSslCert     string `split_words:"true"`
	PGMaxConns    int    `default:"4" split_words:"true"`

	InstrumentationName string `default:"github.com/tjovicic/golang-template/cmd/api"`
}

func NewHandler(ctx context.Context) (*Handler, error) {
	var env handlerConfig

	if err := envconfig.Process("", &env); err != nil {
		return nil, fmt.Errorf("failed to read env variables: %v", err)
	}

	dbClient, err := postgres.NewClient(ctx, &postgres.Config{
		User:        env.PGUser,
		Password:    env.PGPassword,
		Host:        env.PGHost,
		Port:        env.PGPort,
		DBName:      env.PGName,
		SSLRootCert: env.PGSslRootCert,
		SSLKey:      env.PGSslKey,
		SSLCert:     env.PGSslCert,
		MaxConns:    env.PGMaxConns,
	})
	if err != nil {
		return nil, fmt.Errorf("could not connect to database: %v", err)
	}

	if err = dbClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to reach database: %v", err)
	}

	return &Handler{
		config:   env,
		dbClient: dbClient,
	}, nil
}

func (h *Handler) Close() {
	h.dbClient.Close()
}

func (h *Handler) gracefulShutdown(ctx context.Context) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c
		log.Ctx(ctx).Info().Msg("received handler shutdown signal")
		time.Sleep(5 * time.Second)

		h.Close()

		close(c)
	}()
}
