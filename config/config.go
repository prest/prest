package config

// Prest basic config
type Prest struct {
	// HTTPPort Declare which http port the PREST used
	HTTPPort   int    `env:"PREST_HTTP_PORT" envDefault:"3000"`
	PGHost     string `env:"PREST_PG_HOST" envDefault:"127.0.0.1"`
	PGUser     string `env:"PREST_PG_USER"`
	PGPass     string `env:"PREST_PG_PASS"`
	PGDatabase string `env:"PREST_PG_DATABASE"`
	JWTKey     string `env:"PREST_JWT_KEY"`
}
