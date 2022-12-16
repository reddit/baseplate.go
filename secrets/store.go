package secrets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

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
	actual secretStore // if present, all methods are delegated to this interface

	watcher filewatcher.FileWatcher

	secretHandlerFunc SecretHandlerFunc
}

// NewStore returns a new instance of Store by configuring it
// with a filewatcher to watch the file in path for changes ensuring secrets
// store will always return up to date secrets.
//
// Context should come with a timeout otherwise this might block forever, i.e.
// if the path never becomes available.
func NewStore(ctx context.Context, path string, logger log.Wrapper, middlewares ...SecretMiddleware) (*Store, error) {
	store := &Store{
		secretHandlerFunc: nopSecretHandlerFunc,
	}
	store.secretHandler(middlewares...)
	fileInfo, err := os.Stat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	parser := store.parser
	if fileInfo != nil && fileInfo.IsDir() {
		parser = filewatcher.WrapDirParser(store.dirParser)
	}

	result, err := filewatcher.New(
		ctx,
		filewatcher.Config{
			Path:   path,
			Parser: parser,
			Logger: logger,
		},
	)
	if err != nil {
		return nil, err
	}

	store.watcher = result
	return store, nil
}

func (s *Store) parser(r io.Reader) (any, error) {
	secrets, err := NewSecrets(r)
	if err != nil {
		return nil, err
	}

	s.secretHandlerFunc(secrets)

	return secrets, nil
}

func (s *Store) dirParser(dir fs.FS) (any, error) {
	secrets, err := FromDir(dir)
	if err != nil {
		return nil, err
	}

	s.secretHandlerFunc(secrets)

	return secrets, nil
}

// secretHandler creates the middleware chain.
func (s *Store) secretHandler(middlewares ...SecretMiddleware) {
	for _, m := range middlewares {
		s.secretHandlerFunc = m(s.secretHandlerFunc)
	}
}

func (s *Store) getSecrets() *Secrets {
	return s.watcher.Get().(*Secrets)
}

// Close closes the underlying filewatcher and release associated resources.
//
// After Close is called, you won't get any updates to the secret file,
// but can still access the secrets as they were before Close is called.
//
// It's OK to call Close multiple times. Calls after the first one are no-ops.
//
// Close doesn't return non-nil errors, but implements io.Closer.
func (s *Store) Close() error {
	if s.actual != nil {
		return nil
	}
	s.watcher.Stop()
	return nil
}

// AddMiddlewares registers new middlewares to the store.
//
// Every AddMiddlewares call will cause all already registered middlewares to be
// called again with the latest data.
//
// AddMiddlewares call is not thread-safe, it should not be called concurrently.
func (s *Store) AddMiddlewares(middlewares ...SecretMiddleware) {
	if s.actual != nil {
		panic("AddMiddleware cannot be used on a Wrap'd Store")
	}
	s.secretHandler(middlewares...)
	s.secretHandlerFunc(s.getSecrets())
}

// GetSimpleSecret loads secrets from watcher, and fetches a simple secret from secrets
func (s Store) GetSimpleSecret(path string) (SimpleSecret, error) {
	if s.actual != nil {
		return s.actual.GetSimpleSecret(path)
	}
	return s.getSecrets().GetSimpleSecret(path)
}

// GetVersionedSecret loads secrets from watcher, and fetches a versioned secret from secrets
func (s Store) GetVersionedSecret(path string) (VersionedSecret, error) {
	if s.actual != nil {
		return s.actual.GetVersionedSecret(path)
	}
	return s.getSecrets().GetVersionedSecret(path)
}

// GetCredentialSecret loads secrets from watcher, and fetches a credential secret from secrets
func (s Store) GetCredentialSecret(path string) (CredentialSecret, error) {
	if s.actual != nil {
		return s.actual.GetCredentialSecret(path)
	}
	return s.getSecrets().GetCredentialSecret(path)
}

// GetVault returns a struct with a URL and token to access Vault directly. The
// token will have policies attached based on the current EC2 server's Vault
// role. This is only necessary if talking directly to Vault.
//
// This function always returns nil error.
func (s Store) GetVault() (Vault, error) {
	if s.actual != nil {
		return Vault{}, fmt.Errorf("GetVault cannot be used with Wrap'd Store")
	}
	return s.getSecrets().vault, nil
}

// A secretStore is an interface that is satisfied by both v0 and v2 secrets.
type secretStore interface {
	GetSimpleSecret(path string) (SimpleSecret, error)
	GetVersionedSecret(path string) (VersionedSecret, error)
	GetCredentialSecret(path string) (CredentialSecret, error)
}

// Wrap returns a Store that forwards all secret retrieval operations to the given store.
// The returned Store cannot use middleware or provide its Vault information.
func Wrap(store secretStore) *Store {
	return &Store{actual: store}
}
