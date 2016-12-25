package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/file"
	"github.com/mattes/migrate/migrate/direction"
	"github.com/spf13/cobra"
)

var url string
var path string

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Execute migration operations",
	Long:  `Execute migration operations`,
}

func init() {
	RootCmd.AddCommand(migrateCmd)
	migrateCmd.PersistentFlags().StringVar(&url, "url", driverURL(), "Database driver url")
	migrateCmd.PersistentFlags().StringVar(&path, "path", prestConfig.MigrationsPath, "Migrations directory")
}

func driverURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", prestConfig.PGUser, prestConfig.PGPass, prestConfig.PGHost, prestConfig.PGPort, prestConfig.PGDatabase)
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
