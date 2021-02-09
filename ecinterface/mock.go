package ecinterface

import (
	"context"
)

type ecKey struct{}

type ecImpl struct{}

func (ecImpl) ContextToHeader(ctx context.Context) (string, bool) {
	if v := ctx.Value(ecKey{}); v != nil {
		header, ok := v.(string)
		return header, ok
	}
	return "", false
}

func (ecImpl) HeaderToContext(ctx context.Context, header string) (context.Context, error) {
	return context.WithValue(ctx, ecKey{}, header), nil
}

// Mock creates a mocked Interface.
func Mock() Interface {
	return ecImpl{}
}
