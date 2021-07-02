// Package configbp parses configurations per the baseplate spec.
package configbp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/log"
)

// ExpandConfig performs baseplate-conformant configuration expansion.
//
// Currently, this includes expanding $FOO and ${FOO} with their respective environment variables.
func ExpandConfig(rawConfig string) string {
	expanded := os.ExpandEnv(rawConfig)
	if expanded != rawConfig {
		log.Debugf("Expanded configuration:\n%s", expanded)
	}
	return expanded
}

// ParseFile parses configuration from the file at the given path.
//
// Environment variables (e.g. $FOO and ${FOO}) are substituted from the environment before parsing.
// The configuration is parsed into the target, which will typically be a pointer to a struct.
//
// Decoding is performed based on file extension:
//   - YAML (i.e. ".yaml") files are decoded according to ParseYAML.
func ParseFile(path string, ptr interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err // contains filename
	}
	defer f.Close() // safe to blindly close read-only files

	switch ext := filepath.Ext(path); ext {
	case ".yaml":
		return ParseYAML(f, ptr)
	default:
		return fmt.Errorf("unsupported config extension %q", ext)
	}
}

// ParseYAML parses YAML read from the given Reader with strict decoding enabled.
//
// Environment variables (e.g. $FOO and ${FOO}) are substituted from the environment before parsing.
// The configuration is parsed into the target, which will typically be a pointer to a struct.
func ParseYAML(reader io.Reader, ptr interface{}) error {
	rawData := new(strings.Builder)
	if _, err := io.Copy(rawData, reader); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	data := ExpandConfig(rawData.String())

	dec := yaml.NewDecoder(strings.NewReader(data))
	dec.SetStrict(true)
	if err := dec.Decode(ptr); err != nil {
		return fmt.Errorf("parsing YAML into %T: %w", ptr, err)
	}
	log.Debugf("Parsed configuration as %T:\n%#v", ptr, ptr)

	return nil
}
