package edgecontext

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"

	jwt "github.com/reddit/jwt-go/v3"

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
		&AuthenticationToken{},
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
			impl.logger.Log(context.Background(), fmt.Sprintf(
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
				impl.logger.Log(context.Background(), fmt.Sprintf(
					"Failed to parse key #%d: %v",
					i,
					err,
				))
			} else {
				keys = append(keys, key)
			}
		}

		if len(keys) == 0 {
			impl.logger.Log(context.Background(), "No valid keys in secrets store.")
			return
		}

		impl.keysValue.Store(keys)
	}
}
