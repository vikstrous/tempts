package tempts

import "context"

func WrapReturn[T any](fn func(ctx context.Context, t T) error) func(context.Context, T) (struct{}, error) {
	return func(ctx context.Context, t T) (struct{}, error) {
		err := fn(ctx, t)
		return struct{}{}, err
	}
}

func WrapParam[T any](fn func(ctx context.Context) (T, error)) func(context.Context, struct{}) (T, error) {
	return func(ctx context.Context, _ struct{}) (T, error) {
		res, err := fn(ctx)
		return res, err
	}
}

func Wrap(fn func(context.Context) error) func(context.Context, struct{}) (struct{}, error) {
	return func(ctx context.Context, _ struct{}) (struct{}, error) {
		return struct{}{}, fn(ctx)
	}
}
