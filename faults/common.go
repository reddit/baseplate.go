package faults

import "strings"

func GetShortenedAddress(address string) string {
	parts := strings.Split(address, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:2], ".")
}
