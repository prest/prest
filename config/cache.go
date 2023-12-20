package config

// todo:
// - move to cache package
// - insert into server

// Cache structure for storing cache system configuration
type Cache struct {
	Enabled     bool            `mapstructure:"enabled"`
	Time        int             `mapstructure:"time"`
	StoragePath string          `mapstructure:"storagepath"`
	SufixFile   string          `mapstructure:"sufixfile"`
	Endpoints   []CacheEndpoint `mapstructure:"endpoints"`
}

// CacheEndpoint specific configuration for specific endpoint
type CacheEndpoint struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Time     int    `mapstructure:"time"`
}

func (c *Cache) ClearEndpoints() {
	c.Endpoints = []CacheEndpoint{}
}
