package config

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type DatabaseConfig struct {
	Name        string `mapstructure:"name"`
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"pass"`
	Database    string `mapstructure:"database"`
	URL         string `mapstructure:"url"`
	SSLMode     string `mapstructure:"sslmode"`
	SSLCert     string `mapstructure:"sslcert"`
	SSLKey      string `mapstructure:"sslkey"`
	SSLRootCert string `mapstructure:"sslrootcert"`
	MaxIdleConn int    `mapstructure:"maxidleconn"`
	MaxOpenConn int    `mapstructure:"maxopenconn"`
	ConnTimeout int    `mapstructure:"conntimeout"`
	Single      *bool  `mapstructure:"single"`
	Cache       *bool  `mapstructure:"cache"`
}

func boolPtr(b bool) *bool {
	return &b
}

func (d *DatabaseConfig) GetSingle() bool {
	if d.Single != nil {
		return *d.Single
	}
	return viper.GetBool("pg.single")
}

func (d *DatabaseConfig) GetCache() bool {
	if d.Cache != nil {
		return *d.Cache
	}
	return viper.GetBool("pg.cache")
}

func (d *DatabaseConfig) GetConnectionString() string {
	if d.URL != "" {
		return d.URL
	}

	dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=%v connect_timeout=%d",
		d.User, d.Database, d.Host, d.Port, d.SSLMode, d.ConnTimeout)

	if d.Password != "" {
		dbURI += " password=" + d.Password
	}
	if d.SSLCert != "" {
		dbURI += " sslcert=" + d.SSLCert
	}
	if d.SSLKey != "" {
		dbURI += " sslkey=" + d.SSLKey
	}
	if d.SSLRootCert != "" {
		dbURI += " sslrootcert=" + d.SSLRootCert
	}

	return dbURI
}

func (d *DatabaseConfig) ParseURL(connURL string) error {
	u, err := url.Parse(connURL)
	if err != nil {
		return fmt.Errorf("cannot parse database url: %w", err)
	}

	d.Host = u.Hostname()
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return fmt.Errorf("cannot parse database url port: %w", err)
		}
		d.Port = port
	}

	if u.User != nil {
		d.User = u.User.Username()
		if pass, hasPass := u.User.Password(); hasPass {
			d.Password = pass
		}
	}

	d.Database = strings.TrimPrefix(u.Path, "/")

	if sslmode := u.Query().Get("sslmode"); sslmode != "" {
		d.SSLMode = sslmode
	}

	d.URL = connURL
	return nil
}

type namedDatabaseConfig struct {
	Name   string
	Config DatabaseConfig
}

type MultiDBManager struct {
	Databases        map[string]*DatabaseConfig
	OrderedDatabases []namedDatabaseConfig
	DefaultDB        string
	DatabaseCount    int
}

func NewMultiDBManager() *MultiDBManager {
	return &MultiDBManager{
		Databases: make(map[string]*DatabaseConfig),
	}
}

func (m *MultiDBManager) LoadFromConfig() error {
	dbCount := m.getDatabaseCountFromEnv()

	if dbCount > 1 {
		return m.loadFromEnvURLs(dbCount)
	}

	return m.loadFromConfigFile()
}

func (m *MultiDBManager) getDatabaseCountFromEnv() int {
	countStr := os.Getenv("DATABASE_MULTI_NUMBER")
	if countStr == "" {
		if os.Getenv("DATABASE_URL") != "" {
			return 1
		}
		return 0
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0
	}

	m.DatabaseCount = count
	return count
}

func (m *MultiDBManager) loadFromEnvURLs(count int) error {
	for i := 1; i <= count; i++ {
		var connURL string
		var dbName string

		if i == 1 {
			connURL = os.Getenv("DATABASE_URL")
			dbName = os.Getenv("DATABASE_URL_NAME")
			if dbName == "" {
				dbName = "default"
			}
		} else {
			connURL = os.Getenv(fmt.Sprintf("DATABASE_URL%d", i))
			dbName = os.Getenv(fmt.Sprintf("DATABASE_URL%d_NAME", i))
			if dbName == "" {
				dbName = fmt.Sprintf("db%d", i)
			}
		}

		if connURL == "" {
			continue
		}

		dbConfig := &DatabaseConfig{
			Name:        dbName,
			SSLMode:     "disable",
			MaxIdleConn: viper.GetInt("pg.maxidleconn"),
			MaxOpenConn: viper.GetInt("pg.maxopenconn"),
			ConnTimeout: viper.GetInt("pg.conntimeout"),
			Single:      boolPtr(true),
			Cache:       boolPtr(viper.GetBool("pg.cache")),
		}

		if dbConfig.MaxIdleConn == 0 {
			dbConfig.MaxIdleConn = 0
		}
		if dbConfig.MaxOpenConn == 0 {
			dbConfig.MaxOpenConn = 10
		}
		if dbConfig.ConnTimeout == 0 {
			dbConfig.ConnTimeout = 10
		}

		if err := dbConfig.ParseURL(connURL); err != nil {
			return fmt.Errorf("failed to parse DATABASE_URL for %s: %w", dbName, err)
		}

		m.Databases[dbName] = dbConfig
		m.OrderedDatabases = append(m.OrderedDatabases, namedDatabaseConfig{Name: dbName, Config: *dbConfig})

		if m.DefaultDB == "" {
			m.DefaultDB = dbName
		}
	}

	return nil
}

func (m *MultiDBManager) loadFromConfigFile() error {
	var databasesMap map[string]DatabaseConfig
	if err := viper.UnmarshalKey("databases", &databasesMap); err != nil {
		return fmt.Errorf("failed to unmarshal databases config: %w", err)
	}

	if len(databasesMap) == 0 {
		return nil
	}

	var ordered []namedDatabaseConfig
	keys := make([]string, 0, len(databasesMap))
	for name := range databasesMap {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		ordered = append(ordered, namedDatabaseConfig{Name: name, Config: databasesMap[name]})
	}

	for _, entry := range ordered {
		name := entry.Name
		cfg := entry.Config
		cfg.Name = name

		if cfg.Host == "" {
			cfg.Host = viper.GetString("pg.host")
		}
		if cfg.Port == 0 {
			cfg.Port = viper.GetInt("pg.port")
		}
		if cfg.User == "" {
			cfg.User = viper.GetString("pg.user")
		}
		if cfg.Password == "" {
			cfg.Password = viper.GetString("pg.pass")
		}
		if cfg.Database == "" {
			cfg.Database = viper.GetString("pg.database")
		}
		if cfg.SSLMode == "" {
			cfg.SSLMode = viper.GetString("pg.ssl.mode")
		}
		if cfg.MaxIdleConn == 0 {
			cfg.MaxIdleConn = viper.GetInt("pg.maxidleconn")
		}
		if cfg.MaxOpenConn == 0 {
			cfg.MaxOpenConn = viper.GetInt("pg.maxopenconn")
		}
		if cfg.ConnTimeout == 0 {
			cfg.ConnTimeout = viper.GetInt("pg.conntimeout")
		}

		keyPrefix := "databases." + name + "."
		if !viper.IsSet(keyPrefix + "single") {
			cfg.Single = nil
		}
		if !viper.IsSet(keyPrefix + "cache") {
			cfg.Cache = nil
		}

		if cfg.URL != "" {
			if err := cfg.ParseURL(cfg.URL); err != nil {
				return fmt.Errorf("failed to parse URL for database %s: %w", name, err)
			}
		}

		m.Databases[name] = &cfg
		m.OrderedDatabases = append(m.OrderedDatabases, namedDatabaseConfig{Name: name, Config: cfg})

		if m.DefaultDB == "" {
			m.DefaultDB = name
		}
	}

	return nil
}

func (m *MultiDBManager) GetDatabase(name string) (*DatabaseConfig, bool) {
	db, exists := m.Databases[name]
	return db, exists
}

func (m *MultiDBManager) GetDefaultDatabase() (*DatabaseConfig, bool) {
	if m.DefaultDB == "" {
		return nil, false
	}
	return m.GetDatabase(m.DefaultDB)
}

func (m *MultiDBManager) GetDatabaseNames() []string {
	names := make([]string, 0, len(m.Databases))
	for name := range m.Databases {
		names = append(names, name)
	}
	return names
}

func (m *MultiDBManager) HasMultipleDatabases() bool {
	return len(m.Databases) > 1
}
