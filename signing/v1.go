package signing

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"

	"github.com/reddit/baseplate.go/secrets"
)

// V1 implementation.
var V1 Interface = v1{}

// Fixed lengths regarding v1 signatures.
const (
	// The length of the raw, pre-base64-encoding message header.
	V1HeaderLength = 7
	// The length of the raw, pre-base64-encoding signature.
	V1SignatureRawLength = V1HeaderLength + sha256.Size
	// The length of the base64 encoded signature.
	V1SignatureLength = V1SignatureRawLength / 3 * 4
)

type v1 struct{}

type headerV1 struct {
	Version    Version
	_          [2]byte // padding
	Expiration uint32
}

func (v1) Sign(args SignArgs) (sig string, err error) {
	if args.Key.IsEmpty() {
		err = errors.New("signing: empty key")
		return
	}

	now := time.Now()
	expiration := args.ExpiresAt
	if expiration.IsZero() {
		expiration = now.Add(args.ExpiresIn)
	}
	if expiration.Before(now) {
		err = errors.New("signing: already expired")
		return
	}

	header := bytes.NewBuffer(make([]byte, 0, V1HeaderLength))
	err = binary.Write(
		header,
		binary.LittleEndian,
		headerV1{
			Version:    1,
			Expiration: uint32(expiration.Unix()),
		},
	)
	if err != nil {
		return
	}

	raw := make([]byte, V1SignatureRawLength)
	copy(raw, header.Bytes())
	mac := hmac.New(sha256.New, []byte(args.Key))
	mac.Write(header.Bytes())
	mac.Write(args.Message)
	copy(raw[V1HeaderLength:], mac.Sum(nil))
	return base64.URLEncoding.EncodeToString(raw), nil
}

func (v1) Verify(message []byte, signature string, keys ...secrets.Secret) error {
	if len(signature) != V1SignatureLength {
		return VerifyError{
			Data: "signature length mismatch",
		}
	}

	buf, err := base64.URLEncoding.DecodeString(signature)
	if err != nil {
		return VerifyError{
			Cause:  err,
			Reason: VerifyErrorReasonBase64,
		}
	}

	return v1Verify(message, buf, keys, time.Now())
}

func v1Verify(
	message []byte,
	rawSig []byte,
	keys []secrets.Secret,
	now time.Time,
) error {
	if len(rawSig) != V1SignatureRawLength {
		return VerifyError{
			Data: "signature length mismatch",
		}
	}

	var header headerV1
	if err := binary.Read(bytes.NewReader(rawSig), binary.LittleEndian, &header); err != nil {
		return VerifyError{
			Cause: err,
		}
	}
	if header.Version != 1 {
		return VerifyError{
			Reason: VerifyErrorReasonUnknownVersion,
			Data:   header.Version,
		}
	}
	if now.Unix() > int64(header.Expiration) {
		return VerifyError{
			Reason: VerifyErrorReasonExpired,
		}
	}

	for _, key := range keys {
		if key.IsEmpty() {
			continue
		}

		mac := hmac.New(sha256.New, []byte(key))
		mac.Write(rawSig[:V1HeaderLength])
		mac.Write(message)
		expected := mac.Sum(nil)
		if hmac.Equal(rawSig[V1HeaderLength:], expected) {
			return nil
		}
	}
	return VerifyError{
		Reason: VerifyErrorReasonMismatch,
	}
}
