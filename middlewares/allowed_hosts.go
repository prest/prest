package middlewares

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/urfave/negroni/v3"
)

// AllowedHostsMiddleware answers 400 to a request whose Host header names a
// host outside allowedHosts (http.allowed_hosts), so links or cache entries
// built from the Host header cannot be pointed at a host the operator does
// not serve (HTTP Host header attacks).
//
// The host is compared without its port and case-insensitively. An entry
// matches the host exactly; an entry starting with a dot is a subdomain
// wildcard (".example.com" matches example.com and every subdomain of it);
// "*" matches every host. Entries are expected the way config.Parse leaves
// them (no port, no trailing dot, IPv6 addresses in brackets); they are
// lower-cased again here so a list built in code matches the same way.
func AllowedHostsMiddleware(allowedHosts []string) negroni.Handler {
	patterns := make([]string, len(allowedHosts))
	for i, p := range allowedHosts {
		patterns[i] = strings.ToLower(p)
	}
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		if !hostAllowed(requestHost(r.Host), patterns) {
			slog.Warn("request rejected: host not in http.allowed_hosts", "host", r.Host)
			http.Error(w, fmt.Sprintf(jsonErrFormat, "invalid Host header"), http.StatusBadRequest)
			return
		}
		next(w, r)
	})
}

// requestHost returns the host part of a Host header value in the form
// allowed hosts are compared in: lower case, without port or trailing dot, and
// with an IPv6 address kept in brackets.
func requestHost(hostport string) string {
	host := strings.ToLower(hostport)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	}
	return strings.TrimSuffix(host, ".")
}

// hostAllowed reports whether host matches one of patterns. An empty host (an
// HTTP/1.0 request without a Host header) only matches "*".
func hostAllowed(host string, patterns []string) bool {
	for _, p := range patterns {
		switch {
		case p == "*":
			return true
		case host == "":
			continue
		case p == host:
			return true
		case strings.HasPrefix(p, ".") && (strings.HasSuffix(host, p) || host == p[1:]):
			return true
		}
	}
	return false
}
