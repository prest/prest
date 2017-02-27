package config

import "github.com/urfave/negroni"

var app *negroni.Negroni

func initApp() {
	app = negroni.Classic()
}

// GetApp get negroni
func GetApp() *negroni.Negroni {
	if app == nil {
		initApp()
	}
	return app
}
