package edgecontext

import (
	"crypto/rsa"
	"errors"
	"fmt"

	jwt "github.com/reddit/jwt-go/v3"

	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

type keysType = []*rsa.PublicKey

const (
	authenticationPubKeySecretPath = "secret/authentication/public-key"
	jwtAlg                         = "RS256"
)

// ErrNoPublicKeysLoaded is an error returned by ValidateToken indicates that
// the function is called before any public keys are loaded from secrets.
var ErrNoPublicKeysLoaded = errors.New("edgecontext.ValidateToken: no public keys loaded")

// ErrEmptyToken is an error returned by ValidateToken indicates that the JWT
// token is empty string.
var ErrEmptyToken = errors.New("edgecontext.ValidateToken: empty JWT token")

// ValidateToken parses and validates a jwt token, and return the decoded
// AuthenticationToken.
func (impl *Impl) ValidateToken(token string) (*AuthenticationToken, error) {
	keys, ok := impl.keysValue.Load().(keysType)
	if !ok {
		// This would only happen when all previous middleware parsing failed.
		return nil, ErrNoPublicKeysLoaded
	}

	if token == "" {
		// If we don't do the special handling here,
		// jwt.ParseWithClaims below will return an error with message
		// "token contains an invalid number of segments".
		// Also that's still true, it's less obvious what's actually going on.
		// Returning different error for empty token can also help highlighting
		// other invalid tokens that actually causes that invalid number of segments
		// error.
		return nil, ErrEmptyToken
	}

	tok, err := jwt.ParseWithClaims(
		token,
		&AuthenticationToken{
			suppressor: impl.suppressor,
		},
		func(_ *jwt.Token) (interface{}, error) {
			return keys, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if !tok.Valid {
		return nil, jwt.NewValidationError("invalid token", 0)
	}

	if tok.Method.Alg() != jwtAlg {
		return nil, jwt.NewValidationError("wrong signing method", 0)
	}

	if claims, ok := tok.Claims.(*AuthenticationToken); ok {
		return claims, nil
	}

	return nil, jwt.NewValidationError("invalid token type", 0)
}

func (impl *Impl) validatorMiddleware(next secrets.SecretHandlerFunc) secrets.SecretHandlerFunc {
	return func(sec *secrets.Secrets) {
		defer next(sec)

		versioned, err := sec.GetVersionedSecret(authenticationPubKeySecretPath)
		if err != nil {
			log.FallbackWrapper(impl.logger)(fmt.Sprintf(
				"Failed to get secrets %q: %v",
				authenticationPubKeySecretPath,
				err,
			))
			return
		}

		all := versioned.GetAll()
		keys := make(keysType, 0, len(all))
		for i, v := range all {
			key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(v))
			if err != nil {
				log.FallbackWrapper(impl.logger)(fmt.Sprintf(
					"Failed to parse key #%d: %v",
					i,
					err,
				))
			} else {
				keys = append(keys, key)
			}
		}

		if len(keys) == 0 {
			log.FallbackWrapper(impl.logger)("No valid keys in secrets store.")
			return
		}

		impl.keysValue.Store(keys)
	}
}

// JWTErrorSuppressor defines a callback function to check whether a JWT
// validation error can be suppressed/ignored.
//
// If this function returns true,
// then we'll suppress the error in validator and treat it as a valid JWT token.
//
// By default,
// JWTErrorSuppressNone will be used and no errors will be suppressed.
type JWTErrorSuppressor func(e error) bool

// JWTErrorSuppressNone is the default JWTErrorSuppressor to be used.
//
// It always returns false, which means it don't suppress any errors.
func JWTErrorSuppressNone(_ error) bool {
	return false
}

// JWTErrorSuppressExpired suppressed validation errors that the only error is
// because of the claim has expired.
//
// In most cases you shouldn't use it in production environment,
// but it could be useful for test environment handling replayed requests.
func JWTErrorSuppressExpired(e error) bool {
	var err *jwt.ValidationError
	if errors.As(e, &err) {
		// NOTE: err.Errors is a bitfield, but use equal instead of bitwise check,
		// because we only want to suppress the error if that's the only error.
		if err.Errors == jwt.ValidationErrorExpired {
			return true
		}
	}
	return false
}
