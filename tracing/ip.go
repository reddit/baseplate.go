package tracing

import (
	"errors"
	"net"
	"os"
)

// UndefinedIP is used when we fail to get the ip address of this machine.
const UndefinedIP = "undefined"

var errNoIPv4Found = errors.New("no IPv4 ip found")

// get the first IPv4 address that's not loopback.
func getFirstIPv4() (string, error) {
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
	return UndefinedIP, errNoIPv4Found
}
