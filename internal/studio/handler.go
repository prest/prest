package studio

import (
	"embed"
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/prest/prest/v2/helpers"
)

//go:embed all:dist
var distFS embed.FS

const (
	studioPrefix = "/_studio"
	metaPath     = "/_studio/api/meta"
)

func init() {
	_ = mime.AddExtensionType(".js", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
	_ = mime.AddExtensionType(".svg", "image/svg+xml")
	_ = mime.AddExtensionType(".json", "application/json")
	_ = mime.AddExtensionType(".woff2", "font/woff2")
}

// Meta is the allowlisted Studio metadata payload.
type Meta struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	BuildDate   string `json:"buildDate"`
	APIBasePath string `json:"apiBasePath"`
	MCPEndpoint string `json:"mcpEndpoint"`
}

// Handler returns the Studio HTTP handler. When enabled is false, all Studio
// paths return 404. Must be registered before /{database}/{schema} catch-alls.
func Handler(enabled bool) http.Handler {
	content, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("studio: embed dist: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(content))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		setSecurityHeaders(w)

		reqPath := r.URL.Path
		if reqPath == studioPrefix {
			http.Redirect(w, r, studioPrefix+"/", http.StatusFound)
			return
		}
		if reqPath == metaPath || reqPath == metaPath+"/" {
			writeMeta(w, r)
			return
		}
		if !strings.HasPrefix(reqPath, studioPrefix+"/") {
			http.NotFound(w, r)
			return
		}

		rel := strings.TrimPrefix(reqPath, studioPrefix+"/")
		if unescaped, err := url.PathUnescape(rel); err == nil {
			rel = unescaped
		}
		rel = path.Clean(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(rel, "\x00") {
			http.NotFound(w, r)
			return
		}

		// Never SPA-fallback API routes under /_studio/api/
		if rel == "api" || strings.HasPrefix(rel, "api/") {
			http.NotFound(w, r)
			return
		}

		if rel == "" || rel == "." {
			serveIndex(w, r, content)
			return
		}

		if f, err := content.Open(rel); err == nil {
			_ = f.Close()
			if isHashedAsset(rel) {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else if strings.EqualFold(path.Base(rel), "index.html") {
				w.Header().Set("Cache-Control", "no-cache")
			}
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/" + rel
			fileServer.ServeHTTP(w, r2)
			return
		}

		// Client-side route fallback
		serveIndex(w, r, content)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, content fs.FS) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, err := fs.ReadFile(content, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader(string(data)))
}

func writeMeta(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	meta := Meta{
		Version:     helpers.PrestReleaseVersion(),
		Commit:      helpers.PrestCommit(),
		BuildDate:   helpers.PrestBuildDate(),
		APIBasePath: "/",
		MCPEndpoint: "/_mcp",
	}
	_ = json.NewEncoder(w).Encode(meta)
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
}

func isHashedAsset(name string) bool {
	base := path.Base(name)
	// Vite hashed assets: name-HASH.js / name-HASH.css
	if !strings.HasPrefix(path.Dir(name), "assets") && path.Dir(name) != "assets" {
		return false
	}
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return strings.Contains(stem, "-") && len(stem) > 8
}
