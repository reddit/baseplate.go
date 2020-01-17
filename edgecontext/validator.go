package edgecontext

import (
	"errors"

	jwt "gopkg.in/dgrijalva/jwt-go.v3"
)

const (
	authenticationPubKeySecretPath = "secret/authentication/public-key"
	jwtAlg                         = "RS256"
)

// When trying versioned secret with jwt, there are some errors that won't be
// fixed by the next version of the secret, so we can return early instead of
// trying all the remaining versions.
//
// TODO: We can also get rid of this block when upstream added native support
// for key rotation.
var shortCircuitErrors = []uint32{
	jwt.ValidationErrorMalformed,
	jwt.ValidationErrorAudience,
	jwt.ValidationErrorExpired,
	jwt.ValidationErrorIssuedAt,
	jwt.ValidationErrorIssuer,
	jwt.ValidationErrorNotValidYet,
	jwt.ValidationErrorId,
	jwt.ValidationErrorClaimsInvalid,
}

func shouldShortCircutError(err error) bool {
	var ve jwt.ValidationError
	if errors.As(err, &ve) {
		for _, bitmask := range shortCircuitErrors {
			if ve.Errors&bitmask != 0 {
				return true
			}
		}
	}
	return false
}

// ValidateToken parses and validates a jwt token, and return the decoded
// AuthenticationToken.
func ValidateToken(token string) (*AuthenticationToken, error) {
	sec, err := store.GetVersionedSecret(authenticationPubKeySecretPath)
	if err != nil {
		return nil, err
	}

	// TODO 1: Patch upstream to support key rotation natively:
	// https://github.com/dgrijalva/jwt-go/pull/372
	//
	// TODO 2: Use secrets middleware to cache parsed pubkeys.
	var lastErr error
	for _, key := range sec.GetAll() {
		token, err := jwt.ParseWithClaims(
			token,
			&AuthenticationToken{},
			func(_ *jwt.Token) (interface{}, error) {
				return jwt.ParseRSAPublicKeyFromPEM([]byte(key))
			},
		)
		if err != nil {
			if shouldShortCircutError(err) {
				return nil, err
			}
			// Try next pubkey.
			lastErr = err
			continue
		}

		if claims, ok := token.Claims.(*AuthenticationToken); ok && token.Valid {
			return claims, nil
		}

		lastErr = jwt.NewValidationError("", 0)
	}
	return nil, lastErr
}
