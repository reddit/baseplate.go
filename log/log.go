package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	//lint:ignore SA1019 This library is internal only, not actually deprecated
	"github.com/reddit/baseplate.go/internalv2compat"
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
		internalv2compat.SetGlobalLogger(zap.NewNop().Sugar())
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
	internalv2compat.SetGlobalLogger(l.Sugar())
	if Version != "" {
		internalv2compat.SetGlobalLogger(internalv2compat.GlobalLogger().With(zap.String(VersionLogKey, Version)))
	}
	return nil
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	internalv2compat.GlobalLogger().Debug(args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	internalv2compat.GlobalLogger().Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	internalv2compat.GlobalLogger().Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	internalv2compat.GlobalLogger().Error(args...)
}

// DPanic uses fmt.Sprint to construct and log a message.
//
// In development, the logger then panics. (See DPanicLevel for details.)
func DPanic(args ...interface{}) {
	internalv2compat.GlobalLogger().DPanic(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	internalv2compat.GlobalLogger().Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	internalv2compat.GlobalLogger().Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	internalv2compat.GlobalLogger().Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	internalv2compat.GlobalLogger().Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	internalv2compat.GlobalLogger().Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	internalv2compat.GlobalLogger().Errorf(template, args...)
}

// DPanicf uses fmt.Sprintf to log a templated message.
//
// In development, the logger then panics. (See DPanicLevel for details.)
func DPanicf(template string, args ...interface{}) {
	internalv2compat.GlobalLogger().DPanicf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	internalv2compat.GlobalLogger().Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	internalv2compat.GlobalLogger().Fatalf(template, args...)
}

// Debugw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
//
// When debug-level logging is disabled, this is much faster than
//
//	With(keysAndValues).Debug(msg)
func Debugw(msg string, keysAndValues ...interface{}) {
	internalv2compat.GlobalLogger().Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Infow(msg string, keysAndValues ...interface{}) {
	internalv2compat.GlobalLogger().Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Warnw(msg string, keysAndValues ...interface{}) {
	internalv2compat.GlobalLogger().Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context.
//
// The variadic key-value pairs are treated as they are in With.
func Errorw(msg string, keysAndValues ...interface{}) {
	internalv2compat.GlobalLogger().Errorw(msg, keysAndValues...)
}

// DPanicw logs a message with some additional context.
//
// In development, the logger then panics. (See DPanicLevel for details.)
//
// The variadic key-value pairs are treated as they are in With.
func DPanicw(msg string, keysAndValues ...interface{}) {
	internalv2compat.GlobalLogger().DPanicw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context, then panics.
//
// The variadic key-value pairs are treated as they are in With.
func Panicw(msg string, keysAndValues ...interface{}) {
	internalv2compat.GlobalLogger().Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context, then calls os.Exit.
//
// The variadic key-value pairs are treated as they are in With.
func Fatalw(msg string, keysAndValues ...interface{}) {
	internalv2compat.GlobalLogger().Fatalw(msg, keysAndValues...)
}

// Sync flushes any buffered log entries.
func Sync() error {
	return internalv2compat.GlobalLogger().Sync()
}

// With wraps around the underline logger's With.
func With(args ...interface{}) *zap.SugaredLogger {
	return internalv2compat.GlobalLogger().With(args...)
}
