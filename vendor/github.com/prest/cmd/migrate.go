package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/prest/config"
	"github.com/spf13/cobra"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
	"gopkg.in/mattes/migrate.v1/file"
	"gopkg.in/mattes/migrate.v1/migrate/direction"
)

var (
	urlConn string
	path    string
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Execute migration operations",
	Long:  `Execute migration operations`,
}

func driverURL() string {
	// the replace is to maintain compatibility with Go 1.7 https://github.com/golang/go/issues/4013
	// TODO: use url.PathEscape() when stop supporting Go 1.7
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		strings.Replace(url.QueryEscape(config.PrestConf.PGUser), "+", "%20", -1),
		strings.Replace(url.QueryEscape(config.PrestConf.PGPass), "+", "%20", -1),
		strings.Replace(url.QueryEscape(config.PrestConf.PGHost), "+", "%20", -1),
		config.PrestConf.PGPort,
		strings.Replace(url.QueryEscape(config.PrestConf.PGDatabase), "+", "%20", -1),
		url.QueryEscape(config.PrestConf.SSLMode))

}

func writePipe(pipe chan interface{}) (ok bool) {
	okFlag := true
	if pipe != nil {
		for {
			select {
			case item, more := <-pipe:
				if more {
					switch item.(type) {

					case string:
						fmt.Println(item.(string))

					case error:
						c := color.New(color.FgRed)
						c.Println(item.(error).Error())
						okFlag = false

					case file.File:
						f := item.(file.File)
						c := color.New(color.FgBlue)
						if f.Direction == direction.Up {
							c.Print(">")
						} else if f.Direction == direction.Down {
							c.Print("<")
						}
						fmt.Printf(" %s\n", f.FileName)

					default:
						text := fmt.Sprint(item)
						fmt.Println(text)
					}
				} else {
					return okFlag
				}
			}
		}
	}
	return okFlag
}

var timerStart time.Time

func printTimer() {
	diff := time.Now().Sub(timerStart).Seconds()
	if diff > 60 {
		fmt.Printf("\n%.4f minutes\n", diff/60)
	} else {
		fmt.Printf("\n%.4f seconds\n", diff)
	}
}

func verifyMigrationsPath(path string) {
	if path == "" {
		fmt.Println("Please specify path")
		os.Exit(-1)
	}
}
