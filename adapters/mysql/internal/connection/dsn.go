package connection

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// BuildDSN returns a go-sql-driver MySQL DSN.
// multiStatements stays false. prepare false sets interpolateParams so the
// driver sends one text query. prepare true sets interpolateParams false and
// uses the binary prepare protocol. A mysql:// URL cannot turn prepare on.
// The password is escaped into the DSN and must not be logged by callers.
// Client certificate paths fail closed; this version does not register a TLS config.
func BuildDSN(user, pass, host string, port int, database, sslMode, cert, key, rootCert string, prepare bool) (string, error) {
	if user == "" || database == "" {
		return "", fmt.Errorf("mysql user and database are required")
	}
	if host == "" {
		return "", fmt.Errorf("mysql host is required")
	}
	if cert != "" || key != "" || rootCert != "" {
		return "", fmt.Errorf("mysql client certificates are not supported")
	}
	if port == 0 {
		port = 3306
	}
	tlsValue, err := tlsParam(sslMode)
	if err != nil {
		return "", err
	}

	cfg := mysql.NewConfig()
	cfg.User = user
	cfg.Passwd = pass
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", host, port)
	cfg.DBName = database
	cfg.ParseTime = true
	cfg.MultiStatements = false
	interpolate := "true"
	cfg.InterpolateParams = true
	if prepare {
		interpolate = "false"
		cfg.InterpolateParams = false
	}
	cfg.Params = map[string]string{
		"charset":           "utf8mb4",
		"tls":               tlsValue,
		"multiStatements":   "false",
		"interpolateParams": interpolate,
	}
	return cfg.FormatDSN(), nil
}

func tlsParam(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "disable", "false":
		return "false", nil
	case "require", "true":
		return "true", nil
	case "skip-verify", "skipverify":
		return "skip-verify", nil
	default:
		return "", fmt.Errorf("unsupported mysql tls mode %q", mode)
	}
}

// RedactedDSN is a log-safe form of a DSN with the password removed.
func RedactedDSN(dsn string) string {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "mysql://invalid"
	}
	cfg.Passwd = ""
	u := url.URL{Scheme: "mysql", Host: cfg.Addr, Path: "/" + cfg.DBName}
	if cfg.User != "" {
		u.User = url.User(cfg.User)
	}
	return u.String()
}
