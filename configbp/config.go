// Package configbp parses configurations per the baseplate spec.
package configbp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/reddit/baseplate.go/internal/limitopen"
	"github.com/reddit/baseplate.go/log"
)

// BaseplateConfigPath points to the default config file per baseplate.spec.
var BaseplateConfigPath = os.Getenv("BASEPLATE_CONFIG_PATH")

type envsubstReader struct {
	buffer bytes.Buffer
	lines  *bufio.Scanner
}

func (r *envsubstReader) Read(buf []byte) (int, error) {
	// Keep flushing pending data if we have it
	if r.buffer.Len() > 0 {
		return r.buffer.Read(buf)
	}

	// Fill the buffer with some data
	if r.lines.Scan() {
		r.buffer.WriteString(os.ExpandEnv(r.lines.Text()))
		r.buffer.WriteString("\n")
	} else {
		return 0, io.EOF
	}

	// Return some data to satisfy the reader
	return r.buffer.Read(buf)
}

// ParseStrictFile parses configuration from the file at the given path.
//
// Environment variables (e.g. $FOO and ${FOO}) are substituted from the environment before parsing.
// The configuration is parsed into each of the targets, which will typically be pointers to structs.
func ParseStrictFile(path string, ptr interface{}) error {
	f, _, err := limitopen.Open(path)
	if err != nil {
		return err // contains filename
	}
	defer f.Close() // safe to blindly close read-only files

	switch ext := filepath.Ext(path); strings.ToLower(ext) {
	case ".yaml", ".yml":
		return ParseStrictYAML(f, ptr)
	default:
		return fmt.Errorf("unsupported config extension %q", ext)
	}
}

// ParseStrictYAML parses YAML read from the given Reader.
//
// Environment variables (e.g. $FOO and ${FOO}) are substituted from the environment before parsing.
// The configuration is parsed into each of the targets, which will typically be pointers to structs.
func ParseStrictYAML(reader io.Reader, ptr interface{}) error {
	reader = &envsubstReader{
		lines: bufio.NewScanner(reader),
	}

	var debugOutput strings.Builder
	if log.With().Desugar().Core().Enabled(zap.DebugLevel) {
		reader = io.TeeReader(reader, &debugOutput)
	}

	dec := yaml.NewDecoder(reader)
	dec.SetStrict(true)
	if err := dec.Decode(ptr); err != nil {
		// Print out the partial configuration to aid in debugging decode errors now that the file isn't used literally
		if debugOutput.Len() > 0 {
			log.Debugf("Partial configuration for decoding into %T: (error: %s)\n%s", ptr, err, debugOutput.String())
		}
		return fmt.Errorf("parsing YAML into %T: %w", ptr, err)
	}

	// Print out the interpolated YAML configuration in debug mode (which doesn't have JSON logging enabled)
	if debugOutput.Len() > 0 {
		log.Debugf("Parsed configuration as %T:\n%s", ptr, debugOutput.String())
	}

	return nil
}
