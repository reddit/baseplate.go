package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// globalLogger is used by all top-level log methods (e.g. Infof).
//
// Before the Init methods are called, this logger will use a very basic config
// that includes a `pre_init` key to distinguish this condition.
var globalLogger = zap.New(
	zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		os.Stderr,
		zapcore.DebugLevel,
	),
	zap.Fields(
		zap.Bool("pre_init", true),
	),
).WithOptions(
	zap.AddCallerSkip(1), // will always be called via a top-level function from this package
).Sugar()

// Version is the version tag value to be added to the global logger.
//
// If it's changed to non-empty value before the calling of Init* functions
// (InitFromConfig/InitLogger/InitLoggerJSON/InitLoggerWithConfig/InitSentry),
// the global logger initialized will come with a tag of VersionKey("v"),
// added to every line of logs.
// For InitSentry the global sentry will also be initialized with a tag of
// "version".
//
// In order to use it, either set it in your main function early,
// before calling InitLogger* functions,
// with the value coming from flag/config file/etc..
// For example:
//
//     func main() {
//       log.Version = *flagVersion
//       log.InitLoggerJSON(log.Level(*logLevel))
//       // ...
//     }
//
// Or just "stamp" it during build time,
// by passing additional ldflags to go build command.
// For example:
//
//     go build -ldflags "-X github.com/reddit/baseplate.go/log.Version=$(git rev-parse HEAD)"
//
// Change its value after calling Init* functions will have no effects,
// unless you call Init* functions again.
var (
	VersionLogKey = "v"

	Version string
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

// InitLogger provides a quick way to start or replace the global logger.
func InitLogger(logLevel Level) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(logLevel.ToZapLevel())
	config.Encoding = "console"
	config.EncoderConfig.EncodeCaller = ShortCallerEncoder
	config.EncoderConfig.EncodeTime = TimeEncoder
	config.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
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
	config.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// json keys expected by logdna:
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.TimeKey = "timestamp"

	if err := InitLoggerWithConfig(logLevel, config); err != nil {
		// shouldn't happen, but just in case
		panic(err)
	}
}

// InitLoggerWithConfig provides a quick way to start or replace the global
// logger.
//
// Pass in a cfg to provide a logger with custom setting.
//
// This function also wraps the default zap core to convert all int64 and uint64
// fields to strings, to prevent the loss of precision by json log ingester.
// As a result, some of the cfg might get lost during this wrapping, namely
// OutputPaths and ErrorOutputPaths.
func InitLoggerWithConfig(logLevel Level, cfg zap.Config) error {
	if logLevel == NopLevel {
		globalLogger = zap.NewNop().Sugar()
		return nil
	}
	l, err := cfg.Build(
		zap.AddCallerSkip(1),
		zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return wrappedCore{Core: core}
		}),
	)
	if err != nil {
		return err
	}
	globalLogger = l.Sugar()
	if Version != "" {
		globalLogger = globalLogger.With(zap.String(VersionLogKey, Version))
	}
	return nil
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	globalLogger.Debug(args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	globalLogger.Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	globalLogger.Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	globalLogger.Error(args...)
}

// DPanic uses fmt.Sprint to construct and log a message.
//
// In development, the logger then panics. (See DPanicLevel for details.)
func DPanic(args ...interface{}) {
	globalLogger.DPanic(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	globalLogger.Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	globalLogger.Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	globalLogger.Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	globalLogger.Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	globalLogger.Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	globalLogger.Errorf(template, args...)
}

// DPanicf uses fmt.Sprintf to log a templated message.
//
// In development, the logger then panics. (See DPanicLevel for details.)
func DPanicf(template string, args ...interface{}) {
	globalLogger.DPanicf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	globalLogger.Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	globalLogger.Fatalf(template, args...)
}

// Debugw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//
//     With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	globalLogger.Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	globalLogger.Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	globalLogger.Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	globalLogger.Errorw(msg, keysAndValues...)
}

// DPanicw logs a message with some additional context.
//
// In development, the logger then panics. (See DPanicLevel for details.)
//
// The variadic key-value pairs are treated as they are in With.
func DPanicw(msg string, keysAndValues ...interface{}) {
	globalLogger.DPanicw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context, then panics.
//
// The variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	globalLogger.Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context, then calls os.Exit.
//
// The variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	globalLogger.Fatalw(msg, keysAndValues...)
}

// Sync flushes any buffered log entries.
func Sync() error {
	return globalLogger.Sync()
}

// With wraps around the underline logger's With.
func With(args ...interface{}) *zap.SugaredLogger {
	return globalLogger.With(args...)
}
