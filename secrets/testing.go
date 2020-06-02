package secrets

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/reddit/baseplate.go/filewatcher"
)

const (
	// JWTPubKeyPath is the expected key for the EdgeRequestContext public
	// key.
	JWTPubKeyPath = "secret/authentication/public-key"
)

const (
	testVaultURL   = "vault.reddit.ue1.snooguts.net"
	testVaultToken = "17213328-36d4-11e7-8459-525400f56d04"
)

// TestJWTPubKeySecret is the default EdgeRequestContext public key secret set
// when using NewTestSecrets.
//
// pubkey copied from:
// https://github.com/reddit/baseplate.py/blob/db9c1d7cddb1cb242546349e821cad0b0cbd6fce/tests/__init__.py#L12
var TestJWTPubKeySecret = GenericSecret{
	Type:    "versioned",
	Current: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtzMnDEQPd75QZByogNlB\nNY2auyr4sy8UNTDARs79Edq/Jw5tb7ub412mOB61mVrcuFZW6xfmCRt0ILgoaT66\nTp1RpuEfghD+e7bYZ+Q2pckC1ZaVPIVVf/ZcCZ0tKQHoD8EpyyFINKjCh516VrCx\nKuOm2fALPB/xDwDBEdeVJlh5/3HHP2V35scdvDRkvr2qkcvhzoy0+7wUWFRZ2n6H\nTFrxMHQoHg0tutAJEkjsMw9xfN7V07c952SHNRZvu80V5EEpnKw/iYKXUjCmoXm8\ntpJv5kXH6XPgfvOirSbTfuo+0VGqVIx9gcomzJ0I5WfGTD22dAxDiRT7q7KZnNgt\nTwIDAQAB\n-----END PUBLIC KEY-----",
}

func testDocument(raw map[string]GenericSecret) (Document, error) {
	vault := Vault{
		URL:   testVaultURL,
		Token: "17213328-36d4-11e7-8459-525400f56d04",
	}
	if _, ok := raw[JWTPubKeyPath]; !ok {
		raw[JWTPubKeyPath] = TestJWTPubKeySecret
	}
	document := Document{
		Secrets: raw,
		Vault:   vault,
	}
	return document, document.Validate()
}

// NewTestSecrets returns a SecretsStore using the raw map of key to
// GenericSecrets as well as the MockFileWatcher that is used to hold the test
// secrets.
//
// This is provided to aid in testing and should not be used to create production
// secrets.
//
// If you do not provide a value for the key defined by JWTPubKeyPath,
// then we will add a default secret for you.
func NewTestSecrets(ctx context.Context, raw map[string]GenericSecret, middlewares ...SecretMiddleware) (*Store, *filewatcher.MockFileWatcher, error) {
	clone := make(map[string]GenericSecret, len(raw))
	for k, v := range raw {
		clone[k] = v
	}
	document, err := testDocument(clone)
	if err != nil {
		return nil, nil, err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(document); err != nil {
		return nil, nil, err
	}

	store := &Store{
		secretHandlerFunc: nopSecretHandlerFunc,
	}
	store.secretHandler(middlewares...)

	watcher, err := filewatcher.NewMockFilewatcher(&buf, store.parser)
	if err != nil {
		return nil, nil, err
	}

	store.watcher = watcher
	return store, watcher, nil
}

// UpdateTestSecrets replaces the secrets returned by the MockFileWatcher with the
// the given raw secrets.
//
// Like NewTestSecrets, if you do not provide a value for the key defined by
// JWTPubKeyPath, then we will add a default secret for you.
func UpdateTestSecrets(fw *filewatcher.MockFileWatcher, raw map[string]GenericSecret) error {
	document, err := testDocument(raw)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(document); err != nil {
		return err
	}
	return fw.Update(&buf)
}
