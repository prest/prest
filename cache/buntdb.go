package cache

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/avelino/slugify"
	"github.com/tidwall/buntdb"
)

type Cacher interface {
	BuntGet(key string, w http.ResponseWriter) (cacheExist bool)
	BuntSet(key, value string)
}

// getConn connects to database BuntDB - used for caching
func (c *Config) getConn(key string) (db *buntdb.DB, err error) {
	if key != "" {
		// each url will have its own cache,
		// this will avoid slowing down the cache base
		// it is saved in a file on the file system
		key = slugify.Slugify(key)
	}
	db, err = buntdb.Open(filepath.Join(c.StoragePath, fmt.Sprint(key, c.SufixFile)))
	if err != nil {
		// in case of an error to open buntdb the prestd cache is forced to false
		c.Enabled = false
	}
	return
}

// BuntGet downloads the data - if any - that is in the buntdb (embedded cache database)
// using response.URL.String() as key
func (c Config) BuntGet(key string, w http.ResponseWriter) (cacheExist bool) {
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

// BuntSet sets data as cache in buntdb (embedded cache database)
// using response.URL.String() as key
func (c Config) BuntSet(key, value string) {
	uri := strings.Split(key, "?")
	cacheRule, cacheTime := c.EndpointRules(uri[0])
	if !c.Enabled || !cacheRule {
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
