package log

import (
	"context"

	sentry "github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger = zap.NewNop().Sugar()
)

// Level is the verbose representation of log level
type Level string

// Enums for Level
const (
	NopLevel   Level = "nop"
	DebugLevel Level = "debug"
	InfoLevel  Level = "info"
	WarnLevel  Level = "warn"
	ErrorLevel Level = "error"
	PanicLevel Level = "panic"
	FatalLevel Level = "fatal"

	// This will have the same effect as nop but slower
	ZapNopLevel zapcore.Level = zapcore.FatalLevel + 1
)

// ToZapLevel converts Level to a zap acceptable atomic level struct
func (l Level) ToZapLevel() zapcore.Level {
	switch l {
	default:
		return ZapNopLevel
	case DebugLevel:
		return zapcore.DebugLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	case PanicLevel:
		return zapcore.PanicLevel
	case FatalLevel:
		return zapcore.FatalLevel
	}
}

// InitLogger provides a quick way to start or replace a logger.
func InitLogger(logLevel Level) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(logLevel.ToZapLevel())
	config.Encoding = "console"
	config.EncoderConfig.EncodeCaller = ShortCallerEncoder
	config.EncoderConfig.EncodeTime = TimeEncoder
	config.EncoderConfig.EncodeLevel = CapitalLevelEncoder

	if err := InitLoggerWithConfig(logLevel, config); err != nil {
		// shouldn't happen, but just in case
		panic(err)
	}
}

// InitLoggerJSON initializes global logger with full json format.
//
// The JSON format is also compatible with logdna's ingestion format:
// https://docs.logdna.com/docs/ingestion
func InitLoggerJSON(logLevel Level) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(logLevel.ToZapLevel())
	config.Encoding = "json"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	config.EncoderConfig.EncodeTime = JSONTimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// json keys expected by logdna:
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.TimeKey = "timestamp"

	if err := InitLoggerWithConfig(logLevel, config); err != nil {
		// shouldn't happen, but just in case
		panic(err)
	}
}

// InitLoggerWithConfig provides a quick way to start or replace a logger.
//
// Pass in a cfg to provide a logger with custom setting
func InitLoggerWithConfig(logLevel Level, cfg zap.Config) error {
	if logLevel == NopLevel {
		logger = zap.NewNop().Sugar()
		return nil
	}
	l, err := cfg.Build(zap.AddCallerSkip(2))
	if err != nil {
		return err
	}
	logger = l.Sugar()
	return nil
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	logger.Debug(args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	logger.Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	logger.Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	logger.Error(args...)
}

// DPanic uses fmt.Sprint to construct and log a message.
//
// In development, the logger then panics. (See DPanicLevel for details.)
func DPanic(args ...interface{}) {
	logger.DPanic(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	logger.Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	logger.Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	logger.Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	logger.Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	logger.Errorf(template, args...)
}

// DPanicf uses fmt.Sprintf to log a templated message.
//
// In development, the logger then panics. (See DPanicLevel for details.)
func DPanicf(template string, args ...interface{}) {
	logger.DPanicf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	logger.Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	logger.Fatalf(template, args...)
}

// Debugw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//
//     With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	logger.Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	logger.Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	logger.Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	logger.Errorw(msg, keysAndValues...)
}

// DPanicw logs a message with some additional context.
//
// In development, the logger then panics. (See DPanicLevel for details.)
//
// The variadic key-value pairs are treated as they are in With.
func DPanicw(msg string, keysAndValues ...interface{}) {
	logger.DPanicw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context, then panics.
//
// The variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	logger.Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context, then calls os.Exit.
//
// The variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	logger.Fatalw(msg, keysAndValues...)
}

// Sync flushes any buffered log entries.
func Sync() error {
	return logger.Sync()
}

// With wraps around the underline logger's With.
func With(args ...interface{}) *zap.SugaredLogger {
	return logger.With(args...)
}

// ErrorWithSentry logs a message with some additional context,
// then sends the error to Sentry.
//
// The variadic key-value pairs are treated as they are in With.
//
// If a sentry hub is attached to the context object passed in
// (it will be if the context object is from baseplate hooked request context),
// that hub will be used to do the reporting.
// Otherwise the global sentry hub will be used instead.
func ErrorWithSentry(ctx context.Context, msg string, err error, keysAndValues ...interface{}) {
	keysAndValues = append(keysAndValues, "err", err)
	logger.Errorw(msg, keysAndValues...)

	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		hub.CaptureException(err)
	} else {
		sentry.CaptureException(err)
	}
}
