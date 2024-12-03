package main

import (
	"fmt"
	internalContext "github.com/tjovicic/golang-template/internal/context"
	internalHttp "github.com/tjovicic/golang-template/internal/http"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

		fmt.Fprintf(w, "%d", exponentialFibonacci(35))
	}
}

// O(2^n) Fibonacci
func exponentialFibonacci(n int) int {
	// F(0) = 0
	if n == 0 {
		return 0
	}

	// F(1) = 1
	if n == 1 {
		return 1
	}

	// F(n) = F(n-1) + F(n-2) - return the n-th Fibonacci number
	return exponentialFibonacci(n-1) + exponentialFibonacci(n-2)
}
