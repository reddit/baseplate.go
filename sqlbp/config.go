package sqlbp

import (
	"bytes"
	"errors"
	"text/template"
	"time"

	"github.com/reddit/baseplate.go/secrets"
)

// Config is used to configure the SQL client and connection pool.
type Config struct {
	Name                string        `yaml:"name"`
	CredentialsKey      string        `yaml:"credentialsKey"`
	ConnectionString    string        `yaml:"connectionString"`
	MaxIdleConns        int           `yaml:"maxIdleConns"`
	MaxOpenConns        int           `yaml:"maxOpenConns"`
	ConnMaxLifetime     time.Duration `yaml:"connMaxLifetime"`
	PoolMetricsInterval time.Duration `yaml:"poolMetricsInterval"`
}

func (cfg Config) GetConnectionString(s *secrets.Store) (string, error) {
	if cfg.CredentialsKey == "" {
		return cfg.ConnectionString, nil
	}
	if s == nil {
		return "", errors.New("GetConnectionString: nil secrets given")
	}
	tmpl, err := template.New("db." + cfg.Name + ".connection-string").Parse(cfg.ConnectionString)
	if err != nil {
		return "", err
	}
	creds, err := s.GetCredentialSecret(cfg.CredentialsKey)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err = tmpl.Execute(&b, creds); err != nil {
		return "", err
	}
	return b.String(), nil
}
