package core

import (
	"context"
	"io"
)

// Handler is the core abstraction
type Handler interface {
	ServeFlow(ctx context.Context, r io.Reader, w io.Writer) error
}

// HandlerFunc allows regular functions to be used as Handlers
type HandlerFunc func(ctx context.Context, r io.Reader, w io.Writer) error

func (f HandlerFunc) ServeFlow(ctx context.Context, r io.Reader, w io.Writer) error {
	return f(ctx, r, w)
}
