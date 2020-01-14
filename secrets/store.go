package secrets

import (
	"context"
	"fmt"
	"io"

	"github.com/reddit/baseplate.go/filewatcher"
	"github.com/reddit/baseplate.go/log"
)

type (
	// SecretHandlerFunc is the actual function that works with the Secrets
	SecretHandlerFunc func(sec *Secrets)
	// SecretMiddleware creates chain of SecretHandlerFunc calls
	SecretMiddleware func(next SecretHandlerFunc) SecretHandlerFunc
)

func nopSecretHandlerFunc(sec *Secrets) {}

// Store gives access to secret tokens with automatic refresh on change.
//
// This local vault allows access to the secrets cached on disk by the fetcher
// daemon. It will automatically reload the cache when it is changed. Do not
// cache or store the values returned by this class's methods but rather get
// them from this class each time you need them. The secrets are served from
// memory so there's little performance impact to doing so and you will be sure
// to always have the current version in the face of key rotation etc.
type Store struct {
	watcher *filewatcher.Result

	secretHandlerFunc SecretHandlerFunc
}

// NewStore returns a new instance of Store by configuring it
// with a filewatcher to watch the file in path for changes ensuring secrets
// store will always return up to date secrets.
//
// Context should come with a timeout otherwise this might block forever, i.e.
// if the path never becomes available.
func NewStore(ctx context.Context, path string, logger log.Wrapper, middlewares ...SecretMiddleware) (*Store, error) {
	store := &Store{}
	if len(middlewares) > 0 {
		store.secretHandler(middlewares...)
	}

	result, err := filewatcher.New(ctx, path, store.parser, logger)
	if err != nil {
		return nil, err
	}

	store.watcher = result
	return store, nil
}

func (s *Store) parser(r io.Reader) (interface{}, error) {
	secrets, err := NewSecrets(r)
	if err != nil {
		return nil, err
	}

	if s.secretHandlerFunc != nil {
		s.secretHandlerFunc(secrets)
	}

	return secrets, nil
}

// SecretHandler creates the middleware chain.
func (s *Store) secretHandler(middlewares ...SecretMiddleware) {
	if s.secretHandlerFunc == nil {
		s.secretHandlerFunc = nopSecretHandlerFunc
	}

	for _, m := range middlewares {
		s.secretHandlerFunc = m(s.secretHandlerFunc)
	}
}

// GetSimpleSecret loads secrets from watcher, and fetches a simple secret from secrets
func (s *Store) GetSimpleSecret(path string) (SimpleSecret, error) {
	secrets, ok := s.watcher.Get().(*Secrets)
	if !ok {
		return SimpleSecret{}, fmt.Errorf("unexpected type %T", secrets)
	}

	return secrets.GetSimpleSecret(path)
}

// GetVersionedSecret loads secrets from watcher, and fetches a versioned secret from secrets
func (s *Store) GetVersionedSecret(path string) (VersionedSecret, error) {
	secrets, ok := s.watcher.Get().(*Secrets)
	if !ok {
		return VersionedSecret{}, fmt.Errorf("unexpected type %T", secrets)
	}

	return secrets.GetVersionedSecret(path)
}

// GetCredentialSecret loads secrets from watcher, and fetches a credential secret from secrets
func (s *Store) GetCredentialSecret(path string) (CredentialSecret, error) {
	secrets, ok := s.watcher.Get().(*Secrets)
	if !ok {
		return CredentialSecret{}, fmt.Errorf("unexpected type %T", secrets)
	}

	return secrets.GetCredentialSecret(path)
}

// GetVault returns a struct with a URL and token to access Vault directly. The
// token will have policies attached based on the current EC2 server's Vault
// role. This is only necessary if talking directly to Vault.
func (s *Store) GetVault() (Vault, error) {
	var vault Vault
	secrets, ok := s.watcher.Get().(*Secrets)
	if !ok {
		return vault, fmt.Errorf("unexpected type %T", vault)
	}
	return secrets.vault, nil
}
