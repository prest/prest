package cache

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/avelino/slugify"
	"github.com/prest/prest/config"
	"github.com/tidwall/buntdb"
)

// BuntConnect connects to database BuntDB - used for caching
func BuntConnect(key string) (db *buntdb.DB, err error) {
	if key != "" {
		// each url will have its own cache,
		// this will avoid slowing down the cache base
		// it is saved in a file on the file system
		key = slugify.Slugify(key)
	}
	db, err = buntdb.Open(filepath.Join(config.PrestConf.Cache.StoragePath, fmt.Sprint(key, config.PrestConf.Cache.SufixFile)))
	if err != nil {
		// in case of an error to open buntdb the prestd cache is forced to false
		config.PrestConf.Cache.Enabled = false
	}
	return
}

// BuntGet downloads the data - if any - that is in the buntdb (embedded cache database)
// using response.URL.String() as key
func BuntGet(key string, w http.ResponseWriter) (cacheExist bool) {
	db, _ := BuntConnect(key)
	cacheExist = false
	//nolint:errcheck
	db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(key)
		if err == nil {
			cacheExist = true
			http.ResponseWriter.Header(w).Set("Cache-Server", "prestd")
			w.WriteHeader(http.StatusOK)
			http.ResponseWriter.Write(w, []byte(val))
		}
		return nil
	})
	defer db.Close()
	return
}

// BuntSet sets data as cache in buntdb (embedded cache database)
// using response.URL.String() as key
func BuntSet(key, value string) {
	uri := strings.Split(key, "?")
	cacheRule, cacheTime := EndpointRules(uri[0])
	if !config.PrestConf.Cache.Enabled || !cacheRule {
		return
	}
	db, _ := BuntConnect(key)
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
