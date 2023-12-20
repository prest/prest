package cache

// Config structure for storing cache system configuration
type Config struct {
	Enabled     bool       `mapstructure:"enabled"`
	Time        int        `mapstructure:"time"`
	StoragePath string     `mapstructure:"storagepath"`
	SufixFile   string     `mapstructure:"sufixfile"`
	Endpoints   []Endpoint `mapstructure:"endpoints"`
}

// Endpoint specific configuration for specific endpoint
type Endpoint struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Time     int    `mapstructure:"time"`
}

func (c *Config) ClearEndpoints() {
	c.Endpoints = []Endpoint{}
}

// EndpointRules checks if there is a custom caching rule for the endpoint
func (c Config) EndpointRules(uri string) (bool, int) {
	enabled := false
	time := c.Time

	if c.Enabled && len(c.Endpoints) == 0 {
		enabled = true
	}
	for _, endpoint := range c.Endpoints {
		if endpoint.Endpoint == uri {
			enabled = true
			return enabled, endpoint.Time
		}
	}
	return enabled, time
}
