package sqlbp_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/reddit/baseplate.go/secrets"
	"github.com/reddit/baseplate.go/sqlbp"
	"gopkg.in/yaml.v2"
)

func makeTestSecrets(t *testing.T, s secrets.GenericSecret) *secrets.Store {
	raw := map[string]secrets.GenericSecret{
		"db-creds": s,
	}
	store, _, err := secrets.NewTestSecrets(
		context.Background(),
		raw,
	)
	if err != nil {
		t.Fatal(err)
	}

	return store
}

func TestGetConnections(t *testing.T) {
	t.Parallel()

	var secrets = makeTestSecrets(t, secrets.GenericSecret{
		Type:     secrets.CredentialType,
		Username: "db-user",
		Password: "hunter2",
	})

	cases := []struct {
		name      string
		config    sqlbp.Config
		expected  string
		shouldErr bool
		err       string
	}{
		{
			name: "valid conn string",
			config: sqlbp.Config{
				ConnectionString: "user={{ .Username }} password={{ .Password }}",
				CredentialsKey:   "db-creds",
			},
			expected: "user=db-user password=hunter2",
		},
		{
			name: "missing credential",
			config: sqlbp.Config{
				ConnectionString: "user={{ .Username }} password={{ .Password }}",
				CredentialsKey:   "missing-key",
			},
			shouldErr: true,
			expected:  "secrets: no secret has been found for missing-key",
		},
		{
			name: "invalid template",
			config: sqlbp.Config{
				CredentialsKey:   "db-creds",
				ConnectionString: "{{ .Foobar }}",
			},
			shouldErr: true,
			expected:  `template: db..connection-string:1:3: executing "db..connection-string" at <.Foobar>: can't evaluate field Foobar in type secrets.CredentialSecret`,
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				s, err := c.config.GetConnectionString(secrets)
				if err != nil {
					if !c.shouldErr || err.Error() != c.expected {
						t.Errorf("Expecting: %v, got: %v", c.expected, err)
					}
					return
				}
				if s != c.expected {
					t.Errorf("connection string mismatch, expected %s, got: %s\n", c.expected, s)
				}
			},
		)
	}
}

func TestClientConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		raw      string
		expected sqlbp.Config
	}{
		{
			name: "basic-config",
			raw: `
name: "test-db"
poolMetricsInterval: 500ms
maxIdleConns: 40
maxOpenConns: 50
connMaxLifetime: 5m
connectionString: test-conn-string
credentialsKey: secret/ns/db
`,
			expected: sqlbp.Config{
				Name:                "test-db",
				PoolMetricsInterval: time.Millisecond * 500,
				MaxIdleConns:        40,
				MaxOpenConns:        50,
				ConnMaxLifetime:     time.Minute * 5,
				ConnectionString:    "test-conn-string",
				CredentialsKey:      "secret/ns/db",
			},
		},
	}

	for _, _c := range cases {
		c := _c
		t.Run(
			c.name,
			func(t *testing.T) {
				var cfg sqlbp.Config
				if err := yaml.NewDecoder(strings.NewReader(c.raw)).Decode(&cfg); err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(c.expected, cfg) {
					t.Errorf("client config mismatch: \n\nexpected %#v\n\ngot %#v\n\n", c.expected, cfg)
				}
			},
		)
	}
}
