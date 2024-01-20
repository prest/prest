package bunt

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/avelino/slugify"
	"github.com/tidwall/buntdb"

	cf "github.com/prest/prest/config"
)

func New(cfg *cf.CacheConf) *config {
	return &config{
		prestcfg: *cfg,
	}
}

type config struct {
	prestcfg cf.CacheConf
}

// getConn connects to database BuntDB
func (c *config) getConn(key string) (db *buntdb.DB, err error) {
	if key != "" {
		// each url will have its own cache,
		// this will avoid slowing down the cache base
		// it is saved in a file on the file system
		key = slugify.Slugify(key)
	}
	db, err = buntdb.Open(filepath.Join(c.prestcfg.StoragePath, fmt.Sprint(key, c.prestcfg.SufixFile)))
	if err != nil {
		// in case of an error to open buntdb the prestd cache is forced to false
		c.prestcfg.Enabled = false
	}
	return
}

// Get downloads the data - if any - that is in the buntdb (embedded cache database)
// using response.URL.String() as key
func (c config) Get(key string, w http.ResponseWriter) (cacheExist bool) {
	db, err := c.getConn(key)
	if err != nil {
		return
	}
	cacheExist = false
	//nolint:errcheck
	db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(key)
		if err == nil {
			cacheExist = true
			w.Header().Set("Cache-Server", "prestd")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(val))
		}
		return nil
	})
	defer db.Close()
	return
}

// Set sets data as cache in buntdb (embedded cache database)
// using response.URL.String() as key
func (c config) Set(key, value string) {
	uri := strings.Split(key, "?")
	cacheRule, cacheTime := c.EndpointRules(uri[0])
	if !c.prestcfg.Enabled || !cacheRule {
		return
	}
	db, err := c.getConn(key)
	if err != nil {
		return
	}
	//nolint:errcheck
	db.Update(func(tx *buntdb.Tx) error {
		//nolint:errcheck
		tx.Set(key, value,
			&buntdb.SetOptions{
				Expires: true,
				TTL:     time.Duration(cacheTime) * time.Minute})
		return nil
	})
	defer db.Close()
}

func (c *config) ClearEndpoints() {
	c.prestcfg.Endpoints = []cf.Endpoint{}
}

// EndpointRules checks if there is a custom caching rule for the endpoint
func (c config) EndpointRules(uri string) (bool, int) {
	enabled := false
	time := c.prestcfg.Time

	if c.prestcfg.Enabled && len(c.prestcfg.Endpoints) == 0 {
		enabled = true
	}
	for _, endpoint := range c.prestcfg.Endpoints {
		if endpoint.Endpoint == uri {
			enabled = true
			return enabled, endpoint.Time
		}
	}
	return enabled, time
}
