package runtimebp

import (
	"errors"
	"net"
	"os"
)

// UndefinedIP is used when we fail to get the ip address of this machine.
const UndefinedIP = "undefined"

// ErrNoIPv4Found is an error could be returned by GetFirstIPv4 when we can't
// find any non-loopback IPv4 addresses on this machine.
var ErrNoIPv4Found = errors.New("runtimebp: no IPv4 ip found")

// GetFirstIPv4 returns the first local IPv4 address that's not a loopback.
func GetFirstIPv4() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return UndefinedIP, err
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return UndefinedIP, err
	}
	for _, ip := range ips {
		if ip.IsLoopback() {
			continue
		}
		ip = ip.To4()
		if ip == nil {
			// Not an IPv4 IP
			continue
		}
		return ip.String(), nil
	}
	return UndefinedIP, ErrNoIPv4Found
}
