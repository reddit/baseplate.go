// DO NOT EDIT.
// This code was partially generated and then made human readable.
// This approach was adapted from https://github.com/badgerodon/contextaware/blob/4c442dfd39106512496bdd13c42c451da8ddeff3/internal/generate-wrap/main.go
// See also https://www.doxsey.net/blog/fixing-interface-erasure-in-go/ for more context

package httpbp

import (
	"net/http"
)

func wrapResponseWriter(orig, wrapped http.ResponseWriter) http.ResponseWriter {
	var f uint64
	flusher, isFlusher := orig.(http.Flusher)
	if isFlusher { f |= 0x0001 }
	hijacker, isHijacker := orig.(http.Hijacker)
	if isHijacker { f |= 0x0002 }
	pusher, isPusher := orig.(http.Pusher)
	if isPusher { f |= 0x0004 }

	switch f {
	case 0x0000: return wrapped
	case 0x0001: return struct{http.ResponseWriter;http.Flusher}{wrapped, flusher}
	case 0x0002: return struct{http.ResponseWriter;http.Hijacker}{wrapped, hijacker}
	case 0x0003: return struct{http.ResponseWriter;http.Flusher;http.Hijacker}{wrapped, flusher,hijacker}
	case 0x0004: return struct{http.ResponseWriter;http.Pusher}{wrapped, pusher}
	case 0x0005: return struct{http.ResponseWriter;http.Flusher;http.Pusher}{wrapped,flusher,pusher}
	case 0x0006: return struct{http.ResponseWriter;http.Hijacker;http.Pusher}{wrapped, hijacker,pusher}
	case 0x0007: return struct{http.ResponseWriter;http.Flusher;http.Hijacker;http.Pusher}{wrapped, flusher,hijacker,pusher}
	}

	return wrapped
}
