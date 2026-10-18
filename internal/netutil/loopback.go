// Package netutil holds small network helpers shared by several packages.
package netutil

import (
	"net"
	"strings"
)

// IsLoopbackHost reports whether host (a name or IP, without port) refers
// to this machine.
func IsLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]") // "[::1]" -> "::1"
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
