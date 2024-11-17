package context

import (
	"context"
	"github.com/google/uuid"
)

type idKey struct{}

func WithID(parentCtx context.Context) context.Context {
	return context.WithValue(parentCtx, idKey{}, uuid.New().String())
}

func ID(ctx context.Context) string {
	val := ctx.Value(idKey{})
	if val == nil {
		return ""
	}

	id, ok := val.(string)
	if !ok {
		return ""
	}

	return id
}
