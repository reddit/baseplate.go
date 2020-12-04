package log

import (
	"github.com/reddit/zap/zapcore"
)

// KitWrapper is a type that implements go-kit/log.Logger interface with zap logger.
type KitWrapper zapcore.Level

// Log implements go-kit/log.Logger interface.
func (w KitWrapper) Log(keyvals ...interface{}) error {
	const msg = "log.KitWrapper"
	switch zapcore.Level(w) {
	default:
		// for unknown values, fallback to info level.
		fallthrough
	case zapcore.InfoLevel:
		Infow(msg, keyvals...)
	case zapcore.DebugLevel:
		Debugw(msg, keyvals...)
	case zapcore.ErrorLevel:
		Errorw(msg, keyvals...)
	case zapcore.PanicLevel:
		Panicw(msg, keyvals...)
	case zapcore.FatalLevel:
		Fatalw(msg, keyvals...)
	case ZapNopLevel:
		// do nothing
	}
	return nil
}

// KitLogger returns a go-kit compatible logger.
func KitLogger(level Level) KitWrapper {
	return KitWrapper(level.ToZapLevel())
}
