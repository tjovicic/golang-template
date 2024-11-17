package errors

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	internalContext "github.com/tjovicic/golang-template/internal/context"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
)

type Error struct {
	ID      string `json:"id"`
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func InternalHTTPError(ctx context.Context, err error, w http.ResponseWriter, log zerolog.Logger) {
	return &Error{
		Code:    http.StatusInternalServerError,
		Status:  http.StatusText(http.StatusInternalServerError),
		Message: message,
	}
}

func HandleHTTPError(ctx context.Context, err error, w http.ResponseWriter, log zerolog.Logger) {
	var customError *Error

	if !errors.As(err, &customError) {
		customError = InternalHTTPError(err.Error())
	}

	customError.ID = internalContext.ID(ctx)

	span := trace.SpanFromContext(ctx)
	span.RecordError(customError)
	span.SetStatus(codes.Error, customError.Message)
	log.ErrorWithJSON("error", []byte(customError.Error()), httpStatusErrMsg)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(customError.Code)
	fmt.Fprintln(w, err.Error())
}
