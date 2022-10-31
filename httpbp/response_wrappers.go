package httpbp

import (
	"net/http"
)

type optionalResponseWriter uint64

const (
	flusher optionalResponseWriter = 1 << iota
	hijacker
	pusher
)

func wrapResponseWriter(orig, wrapped http.ResponseWriter) http.ResponseWriter {
	var f optionalResponseWriter
	fl, isFlusher := orig.(http.Flusher)
	if isFlusher {
		f |= flusher
	}
	h, isHijacker := orig.(http.Hijacker)
	if isHijacker {
		f |= hijacker
	}
	p, isPusher := orig.(http.Pusher)
	if isPusher {
		f |= pusher
	}

	switch f {
	case 0:
		return wrapped
	case flusher:
		return struct {
			http.ResponseWriter
			http.Flusher
		}{wrapped, fl}
	case hijacker:
		return struct {
			http.ResponseWriter
			http.Hijacker
		}{wrapped, h}
	case flusher | hijacker:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Hijacker
		}{wrapped, fl, h}
	case pusher:
		return struct {
			http.ResponseWriter
			http.Pusher
		}{wrapped, p}
	case flusher | pusher:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Pusher
		}{wrapped, fl, p}
	case hijacker | pusher:
		return struct {
			http.ResponseWriter
			http.Hijacker
			http.Pusher
		}{wrapped, h, p}
	case flusher | hijacker | pusher:
		return struct {
			http.ResponseWriter
			http.Flusher
			http.Hijacker
			http.Pusher
		}{wrapped, fl, h, p}
	}

	return wrapped
}
