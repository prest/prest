package config

import (
	"log/slog"
	"net"
	"strings"

	"github.com/spf13/viper"
)

// parseAllowedHosts reads http.allowed_hosts: the host names pREST serves. The
// environment (PREST_HTTP_ALLOWED_HOSTS) takes a comma-separated list, the
// config file an array or a comma-separated string. Every entry is normalized
// with normalizeAllowedHost and empty entries are dropped. nil means unset, and
// the Host header is then not checked. A value that names entries but leaves
// none usable (".", ":8080") returns an empty, non-nil list, which serves no
// host: a broken setting must not silently turn the check off.
func parseAllowedHosts(v *viper.Viper) []string {
	const key = "http.allowed_hosts"
	var raw []string
	if s, ok := v.Get(key).(string); ok {
		raw = strings.Split(s, ",")
	} else {
		raw = v.GetStringSlice(key)
	}
	var hosts []string
	invalid := false
	for _, entry := range raw {
		if host := normalizeAllowedHost(entry); host != "" {
			hosts = append(hosts, host)
		} else if strings.TrimSpace(entry) != "" {
			invalid = true
			slog.Error("http.allowed_hosts entry is not a host name, ignoring it", "entry", entry)
		}
	}
	if hosts == nil && invalid {
		return []string{}
	}
	return hosts
}

// normalizeAllowedHost puts one http.allowed_hosts entry in the form the
// request Host is compared in: trimmed, lower case, without a trailing dot, and
// with an IPv6 address in brackets ("::1" becomes "[::1]"). Entries are host
// names, so a port is dropped with a warning instead of leaving an entry that
// no request could match. "*" and leading-dot subdomain wildcards
// (".example.com") are kept as they are.
func normalizeAllowedHost(entry string) string {
	host := strings.ToLower(strings.TrimSpace(entry))
	if host == "" || host == "*" {
		return host
	}
	if h, port, err := net.SplitHostPort(host); err == nil {
		slog.Warn("http.allowed_hosts entries are matched without the port, ignoring it",
			"entry", entry, "port", port)
		host = h
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
	} else if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		host = "[" + host + "]"
	}
	return strings.TrimSuffix(host, ".")
}
