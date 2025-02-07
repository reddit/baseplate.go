package headerbp

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/signing"
)

const delimiter = rune(0)

var signatureVersionPrefix = strconv.Itoa(signatureVersion)

var ErrInvalidSignatureVersion = fmt.Errorf("invalid  version")

type headerSignatureContextKey struct{}

func setSignatureOnContext(ctx context.Context, sig string) context.Context {
	return context.WithValue(ctx, headerSignatureContextKey{}, sig)
}

// HeaderSignatureFromContext gets the header signature from the context. This can be used in client middleware to propagate the
// signature along with the headers if they are unchanged by the request.
func HeaderSignatureFromContext(ctx context.Context) (string, bool) {
	sig, ok := ctx.Value(headerSignatureContextKey{}).(string)
	return sig, ok
}

var messageBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	buf := messageBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func putBuffer(buf *bytes.Buffer) {
	messageBufferPool.Put(buf)
}

func concatHeaders(b *bytes.Buffer, headerNames []string, getHeader func(string) string, versionPrefix string) {
	type manifestEntry struct {
		// normalized is used to sort the manifest and as the header name in the message
		normalized string

		// original is maintained in order to potentially reduce the number of allocations when looking up the header
		// value
		original string
	}
	manifest := make([]manifestEntry, 0, len(headerNames))
	for _, k := range headerNames {
		manifest = append(manifest, manifestEntry{
			// do not cache the normalized key here though since we do not know if we can trust it, and we don't want to
			// cache junk headers.
			normalized: normalizeKey(k, false),
			original:   k,
		})
	}
	slices.SortFunc(manifest, func(i, j manifestEntry) int {
		if i.normalized < j.normalized {
			return -1
		} else if i.normalized > j.normalized {
			return 1
		} else {
			return 0
		}
	})

	b.WriteString(versionPrefix)
	b.WriteRune(delimiter)
	// include the number of headers to protect against a case where someone tries to embed portions of the message
	// into a header value.
	b.WriteString(strconv.Itoa(len(manifest)))
	b.WriteRune(delimiter)
	for _, entry := range manifest {
		b.WriteString(entry.normalized)
		b.WriteRune(':')
		b.WriteString(getHeader(entry.original))
		b.WriteRune(delimiter)
	}
}

type signHeadersOptions func()

// SignHeaders signs the given headers with the given signing secret using baseplate message signing. The
// signature will be valid for 5 minutes.
//
// This can be used by middlewares clients that send requests to other services that are exposed to untrusted traffic to
// sign the headers before sending them or by services that are setting up the initial headers to be propagated.
func SignHeaders(
	ctx context.Context,
	signingSecret secrets.VersionedSecret,
	headerNames []string,
	getHeader func(string) string,
	opts ...signHeadersOptions,
) (string, error) {
	b := getBuffer()
	defer putBuffer(b)

	concatHeaders(b, headerNames, getHeader, signatureVersionPrefix)
	signature, err := signing.Sign(
		signing.SignArgs{
			Message:   b.Bytes(),
			Secret:    signingSecret,
			ExpiresIn: 5 * time.Minute,
		})
	if err != nil {
		return "", fmt.Errorf("generating signature: %w", err)
	}

	b.Reset()
	b.WriteString(signatureVersionPrefix)
	b.WriteRune('.')
	b.WriteString(signature)
	return b.String(), nil
}

type signatureComponents struct {
	version       int
	versionPrefix string
	signature     string
}

func extractVersion(signature string) (*signatureComponents, error) {
	version, sig, ok := strings.Cut(signature, ".")
	if !ok {
		return nil, fmt.Errorf("no version found in signature")
	}
	versionInt, err := strconv.Atoi(version)
	if err != nil {
		return nil, fmt.Errorf("invalid version format %q: %w", version, err)
	}
	return &signatureComponents{
		version:       versionInt,
		versionPrefix: version,
		signature:     sig,
	}, nil
}

// VerifyHeaders verifies the signature of the given headers using the given verification secret. If the signature
// is valid, it sets the signature on the context.
//
// This can be used by middlewares that receive requests from untrusted traffic to verify the headers before recording
// them to be propagated.
func VerifyHeaders(
	ctx context.Context,
	verificationSecret secrets.VersionedSecret,
	signature string,
	headerNames []string,
	getHeader func(string) string,
) (context.Context, error) {
	components, err := extractVersion(signature)
	if err != nil {
		return ctx, fmt.Errorf("%w: %w", ErrInvalidSignatureVersion, err)
	}
	if components.version != 1 {
		return ctx, fmt.Errorf("%w: unsupported version number %d", ErrInvalidSignatureVersion, components.version)
	}

	b := getBuffer()
	defer putBuffer(b)
	concatHeaders(b, headerNames, getHeader, components.versionPrefix)
	if err := signing.Verify(b.Bytes(), components.signature, verificationSecret); err != nil {
		return ctx, fmt.Errorf("verification error: %w", err)
	}
	ctx = setV2SignatureContext(ctx, signature)
	return setSignatureOnContext(ctx, signature), nil
}
