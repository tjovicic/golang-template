package telemetry

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	meternoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

// InstallTraceExportPipeline installs global trace provider.
func InstallTraceExportPipeline(ctx context.Context, config TraceConfig) (func(context.Context), error) {
	var tracerProvider trace.TracerProvider

	tracerProvider, cleanupFunc, err := newTracerProvider(ctx, config)
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return cleanupFunc, nil
}

// InstallMetricExportPipeline installs global metric provider.
func InstallMetricExportPipeline(ctx context.Context, config MeterConfig) (func(context.Context), error) {
	var meterProvider metric.MeterProvider

	meterProvider, cleanupFunc, err := NewMeterProvider(ctx, config)
	if err != nil {
		return nil, err
	}

	otel.SetMeterProvider(meterProvider)

	return cleanupFunc, nil
}

func newTracerProvider(ctx context.Context, config TraceConfig) (trace.TracerProvider, func(context.Context), error) {
	if config.CollectorURL == "" || config.Environment == "" {
		return tracenoop.NewTracerProvider(), func(context.Context) {}, nil
	}

	collectorURL := fmt.Sprintf("dns:///%s", config.CollectorURL)
	conn, err := grpc.NewClient(
		collectorURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// grpc.WithKeepaliveParams(kacp),
		// grpc.WithDefaultServiceConfig(sc),
	)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to create gRPC connection to collector, creating noop tracer")
		return tracenoop.NewTracerProvider(), func(context.Context) {}, nil //nolint:nilerr // ignore err check
	}

	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := newResource(ctx, config.Config)
	if err != nil {
		return nil, nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)

	cleanupFunc := func(shutdownCtx context.Context) {
		// Shutdown will flush any remaining spans and shut down the exporter.
		if err = tracerProvider.Shutdown(shutdownCtx); err != nil {
			log.Ctx(shutdownCtx).Err(err).Msg("stopping tracer provider")
		}

		if err = exp.Shutdown(shutdownCtx); err != nil {
			log.Ctx(shutdownCtx).Err(err).Msg("shutting down trace exporter")
		}
	}

	return tracerProvider, cleanupFunc, nil
}

func NewMeterProvider(ctx context.Context, config MeterConfig) (metric.MeterProvider, func(context.Context), error) {
	if config.CollectorURL == "" || config.Environment == "" {
		return meternoop.NewMeterProvider(), func(context.Context) {}, nil
	}

	collectorURL := fmt.Sprintf("dns:///%s", config.CollectorURL)
	conn, err := grpc.NewClient(
		collectorURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// grpc.WithKeepaliveParams(kacp),
		// grpc.WithDefaultServiceConfig(sc),
	)
	if err != nil {
		log.Ctx(ctx).Err(err).Msg("failed to create gRPC connection to collector, creating noop meter")
		return meternoop.NewMeterProvider(), func(context.Context) {}, nil //nolint:nilerr // ignore err check
	}

	exp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, nil, err
	}

	res, err := newResource(ctx, config.Config)
	if err != nil {
		return nil, nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(config.ExportInterval))),
		sdkmetric.WithResource(res),
	)

	cleanupFunc := func(shutdownCtx context.Context) {
		// pushes any last exports to the receiver
		if err = meterProvider.Shutdown(shutdownCtx); err != nil {
			log.Ctx(shutdownCtx).Err(err).Msg("stopping metric provider")
		}

		if err = exp.Shutdown(shutdownCtx); err != nil {
			log.Ctx(shutdownCtx).Err(err).Msg("shutting down metric exporter")
		}
	}

	return meterProvider, cleanupFunc, nil
}

func newResource(ctx context.Context, config Config) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.Name),
			semconv.ServiceVersionKey.String(config.Version),
			semconv.DeploymentNameKey.String(config.Environment),
		),
	)
}

type TraceConfig struct {
	Config
}

type MeterConfig struct {
	Config

	// ExportInterval configures the intervening time between exports.
	// This option overrides any value set for the OTEL_METRIC_EXPORT_INTERVAL environment variable.
	// If this option is not used or d is less than or equal to zero, 60 seconds is used as the default.
	ExportInterval time.Duration
}

type Config struct {
	Name            string
	Version         string
	Environment     string
	CollectorURL    string
	ShutdownTimeout time.Duration
}
