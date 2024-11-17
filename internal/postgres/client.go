package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	instrumentationName = "github.com/tjovicic/golang-template/postgres"
)

var (
	tracer          = otel.Tracer(instrumentationName)
	traceAttributes = []attribute.KeyValue{
		{Key: "db.system", Value: attribute.StringValue("postgresql")},
	}
)

func NewClient(ctx context.Context, config *Config) (*pgxpool.Pool, error) {
	uri := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", config.User, config.Password, config.Host, config.Port, config.DBName)

	if config.SSLRootCert != "" && config.SSLKey != "" && config.SSLCert != "" {
		uri += fmt.Sprintf("?sslrootcert=%s&sslkey=%s&sslcert=%s", config.SSLRootCert, config.SSLKey, config.SSLCert)
	} else {
		uri += "?sslmode=disable"
	}

	conn, err := pgxpool.New(ctx, uri)
	if err != nil {
		return nil, err
	}

	conn.Config().MaxConns = int32(config.MaxConns)

	return conn, nil
}

type Config struct {
	User     string
	Password string
	Host     string
	Port     string
	DBName   string

	SSLRootCert string
	SSLKey      string
	SSLCert     string

	MaxConns int
}
