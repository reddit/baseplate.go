package transport

import (
	"errors"
	"fmt"
	"strings"
)

/**
 * DTabs are strings in the following form:
 * service.from=>service.to;another-service.from=>another-service.to
 */
func ParseDTabs(unparsed string) (map[string]string, error) {
	dentries := filterDEntries(strings.Split(unparsed, ";"))
	result := make(map[string]string)
	for _, unparsedDEntry := range dentries {
		from, to, err := parseDEntry(unparsedDEntry)
		if err != nil {
			return nil, err
		}
		result[from] = to
	}
	return result, nil
}

func parseDEntry(unparsed string) (string, string, error) {
	overrides := strings.Split(unparsed, "=>")
	if len(overrides) != 2 {
		return "", "", errors.New(fmt.Sprintf("malformed DEntry %s", unparsed))
	}
	return strings.TrimSpace(overrides[0]), strings.TrimSpace(overrides[1]), nil
}

// This makes sure we drop the trailing semi-colon
func filterDEntries(dentries []string) []string {
	res := make([]string, 0)
	for _, dentry := range dentries {
		trimmed := strings.TrimSpace(dentry)
		if len(trimmed) != 0 {
			res = append(res, trimmed)
		}
	}
	return res
}
