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
	var w optionalResponseWriter
	f, isFlusher := orig.(http.Flusher)
	if isFlusher {
		w |= flusher
	}
	h, isHijacker := orig.(http.Hijacker)
	if isHijacker {
		w |= hijacker
	}
	p, isPusher := orig.(http.Pusher)
	if isPusher {
		w |= pusher
	}

	switch w {
	case 0:
		return wrapped
	case flusher:
		return struct {
			http.ResponseWriter
			http.Flusher
		}{wrapped, f}
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
		}{wrapped, f, h}
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
		}{wrapped, f, p}
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
		}{wrapped, f, h, p}
	}

	return wrapped
}
