package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
)

func main() {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("unable to resolve current file path")
	}

	baseDir := filepath.Dir(currentFile)
	staticDir := filepath.Join(baseDir, "html")
	swaggerFile := filepath.Join(baseDir, "swagger.json")

	mux := http.NewServeMux()

	// Serve static files from ./html.
	mux.Handle("/", http.FileServer(http.Dir(staticDir)))

	// Serve Swagger UI page.
	mux.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/swagger" && r.URL.Path != "/swagger/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Swagger UI</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.onload = () => {
        window.ui = SwaggerUIBundle({
          url: '/swagger.json',
          dom_id: '#swagger-ui',
        });
      };
    </script>
  </body>
</html>`)
	})

	// Serve the local Swagger document.
	mux.HandleFunc("/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, swaggerFile)
	})

	log.Println("listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
