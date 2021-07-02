// Package configbp parses configurations per the baseplate spec.
package configbp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/reddit/baseplate.go/log"
	"gopkg.in/yaml.v2"
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
// The configuration is parsed into each of the targets, which will typically be pointers to structs.
func ParseFile(path string, ptrs ...interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err // contains filename
	}
	defer f.Close() // safe to blindly close read-only files

	switch ext := filepath.Ext(path); ext {
	case ".yaml":
		return ParseYAML(f, ptrs...)
	default:
		return fmt.Errorf("unsupported config extension %q", ext)
	}
}

// ParseYAML parses YAML read from the given Reader.
//
// Environment variables (e.g. $FOO and ${FOO}) are substituted from the environment before parsing.
// The configuration is parsed into each of the targets, which will typically be pointers to structs.
func ParseYAML(reader io.Reader, ptrs ...interface{}) error {
	rawData := new(strings.Builder)
	if _, err := io.Copy(rawData, reader); err != nil {
		return fmt.Errorf("reading config: %w", err)
	}

	data := ExpandConfig(rawData.String())

	for _, ptr := range ptrs {
		dec := yaml.NewDecoder(strings.NewReader(data))
		dec.SetStrict(true)
		if err := dec.Decode(ptr); err != nil {
			return fmt.Errorf("parsing YAML into %T: %w", ptr, err)
		}
		log.Debugf("Parsed configuration as %T:\n%#v", ptr, ptr)
	}

	return nil
}
