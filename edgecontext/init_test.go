package edgecontext_test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/reddit/baseplate.go/edgecontext"
	"github.com/reddit/baseplate.go/log"
	"github.com/reddit/baseplate.go/secrets"
)

// pubkey copied from https://github.com/reddit/baseplate.py/blob/db9c1d7cddb1cb242546349e821cad0b0cbd6fce/tests/__init__.py#L12
const secretStore = `
{
	"secrets": {
		"secret/authentication/public-key": {
			"type": "versioned",
			"current": "foobar",
			"previous": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtzMnDEQPd75QZByogNlB\nNY2auyr4sy8UNTDARs79Edq/Jw5tb7ub412mOB61mVrcuFZW6xfmCRt0ILgoaT66\nTp1RpuEfghD+e7bYZ+Q2pckC1ZaVPIVVf/ZcCZ0tKQHoD8EpyyFINKjCh516VrCx\nKuOm2fALPB/xDwDBEdeVJlh5/3HHP2V35scdvDRkvr2qkcvhzoy0+7wUWFRZ2n6H\nTFrxMHQoHg0tutAJEkjsMw9xfN7V07c952SHNRZvu80V5EEpnKw/iYKXUjCmoXm8\ntpJv5kXH6XPgfvOirSbTfuo+0VGqVIx9gcomzJ0I5WfGTD22dAxDiRT7q7KZnNgt\nTwIDAQAB\n-----END PUBLIC KEY-----"
		}
	}
}`

func TestMain(m *testing.M) {
	log.InitLogger(log.DebugLevel)

	dir, err := ioutil.TempDir("", "edge_context_test_")
	if err != nil {
		log.Panic(err)
	}
	defer os.RemoveAll(dir)

	tmpFile, err := ioutil.TempFile(dir, "secrets.json")
	if err != nil {
		log.Panic(err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write([]byte(secretStore)); err != nil {
		log.Panic(err)
	}
	if err := tmpFile.Close(); err != nil {
		log.Panic(err)
	}

	store, err := secrets.NewStore(context.Background(), tmpPath, nil)
	if err != nil {
		log.Panic(err)
	}

	edgecontext.Init(edgecontext.Config{Store: store})
	os.Exit(m.Run())
}
