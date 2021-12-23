package cache

import (
	"net/http"
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
	db, err = buntdb.Open(config.PrestConf.CacheStoragePath + key + config.PrestConf.CacheSufixFile)
	if err != nil {
		// in case of an error to open buntdb the prestd cache is forced to false
		config.PrestConf.Cache = false
	}
	return
}

// BuntGet downloads the data - if any - that is in the buntdb (embeded cache database)
// using response.URL as key
func BuntGet(key string, w http.ResponseWriter) (cacheExist bool) {
	db, _ := BuntConnect(key)
	cacheExist = false
	db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(key)
		if err == nil {
			cacheExist = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(val))
		}
		return nil
	})
	defer db.Close()
	return
}

// BuntSet sets data as cache in buntdb (embeded cache database)
// using response.URL as key
func BuntSet(key, value string) {
	if !config.PrestConf.Cache {
		return
	}
	db, _ := BuntConnect(key)
	db.Update(func(tx *buntdb.Tx) error {
		tx.Set(key, value,
			&buntdb.SetOptions{
				Expires: true,
				TTL:     time.Duration(config.PrestConf.CacheTime) * time.Minute})
		return nil
	})
	defer db.Close()
}
