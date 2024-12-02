package main

import (
	internalContext "github.com/tjovicic/golang-template/internal/context"
	internalHttp "github.com/tjovicic/golang-template/internal/http"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"math"
	"net/http"
)

func GetHandler(h *internalHttp.Handler) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			ctx = r.Context()
			// log = zerolog.Ctx(ctx)
			span = trace.SpanFromContext(ctx)
			id   = internalContext.ID(ctx)
		)

		span.SetAttributes(attribute.String("id", id))
		defer span.End()

		w.Header().Set("Content-Type", "application/json")

		for i := 0; i < 1000000; i++ {
			math.Pow(36, 89)
		}

		if _, err := w.Write([]byte("hello world")); err != nil {
			// internalErrors.HandleHTTPError(ctx, err, w, log)
		}
	}
}
