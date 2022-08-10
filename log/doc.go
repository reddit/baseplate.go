// Package log provides a wrapped zap logger interface for microservices to use,
// and also a simple Wrapper interface to be used by other Baseplate.go packages.
//
// For zap logger related features,
// we provide both a global logger which can be used by top level functions,
// and a way to attach logger with additional info (e.g. trace id) to context
// object and reuse.
// When you need to use the zap logger and you have a context object,
// you should use the logger attached to the context, like:
//
//	log.C(ctx).Errorw("Something went wrong!", "err", err)
//
// But if you don't have a context object,
// instead of creating one to use logger, you should use the global one:
//
//	log.Errorw("Something went wrong!", "err", err)
package log
