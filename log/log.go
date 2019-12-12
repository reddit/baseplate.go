package log

import (
	"github.com/getsentry/raven-go"
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

// InitLogger provides a quick way to start or replace a logger
// Pass in debug as log level enables development mode (which makes DPanicLevel logs panic).
func InitLogger(logLevel Level) {
	if logLevel == NopLevel {
		logger = zap.NewNop().Sugar()
		return
	}

	var config = zap.NewProductionConfig()
	lvl := StringToAtomicLevel(logLevel)
	config.Encoding = "console"
	config.EncoderConfig.EncodeCaller = ShortCallerEncoder
	config.EncoderConfig.EncodeTime = TimeEncoder
	config.EncoderConfig.EncodeLevel = CapitalLevelEncoder
	config.Level = zap.NewAtomicLevelAt(lvl)

	if lvl == zap.DebugLevel {
		config.Development = true
	}
	// TODO: error handling
	l, _ := config.Build(zap.AddCallerSkip(2))
	logger = l.Sugar()
}

// InitLoggerWithConfig provides a quick way to start or replace a logger
// Pass in debug as log level enables development mode (which makes DPanicLevel logs panic).
// Pass in a cfg to provide a logger with custom setting
func InitLoggerWithConfig(logLevel Level, cfg zap.Config) {
	if logLevel == NopLevel {
		logger = zap.NewNop().Sugar()
		return
	}
	// TODO: error handling
	l, _ := cfg.Build(zap.AddCallerSkip(2))
	logger = l.Sugar()
}

// StringToAtomicLevel converts in to a zap acceptable atomic level struct
func StringToAtomicLevel(loglevel Level) zapcore.Level {
	switch loglevel {
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
	default:
		return ZapNopLevel
	}
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

// ErrorWithRaven logs a message with some additional context, then sends the error to Sentry.
// The variadic key-value pairs are treated as they are in With.
func ErrorWithRaven(msg string, err error, keysAndValues ...interface{}) {
	keysAndValues = append(keysAndValues, "err", err)
	logger.Errorw(msg, keysAndValues...)
	raven.CaptureError(err, nil)
}

// DPanic uses fmt.Sprint to construct and log a message. In development, the
// logger then panics. (See DPanicLevel for details.)
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

// DPanicf uses fmt.Sprintf to log a templated message. In development, the
// logger then panics. (See DPanicLevel for details.)
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

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//  s.With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	logger.Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	logger.Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	logger.Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	logger.Errorw(msg, keysAndValues...)
}

// DPanicw logs a message with some additional context. In development, the
// logger then panics. (See DPanicLevel for details.) The variadic key-value
// pairs are treated as they are in With.
func DPanicw(msg string, keysAndValues ...interface{}) {
	logger.DPanicw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context, then panics. The
// variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	logger.Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context, then calls os.Exit. The
// variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	logger.Fatalw(msg, keysAndValues...)
}

// Sync flushes any buffered log entries.
func Sync() error {
	return logger.Sync()
}

// With wraps around the underline logger's With
func With(args ...interface{}) *zap.SugaredLogger {
	return logger.With(args...)
}
