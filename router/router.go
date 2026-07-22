package router

import (
	"net/http"
	"runtime"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/internal/studio"
	"github.com/prest/prest/v2/middlewares"
	"github.com/prest/prest/v2/plugins"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// RegisterRoutes wires HTTP routes onto the given router.
func RegisterRoutes(
	router *mux.Router,
	cfg *config.Prest,
	h *controllers.Handlers,
	crudStack *middlewares.CRUDStack,
	queryStack *middlewares.QueryStack,
	adminStack *middlewares.AdminQueryStack,
	plg *plugins.Plugins,
) {
	// When telemetry is enabled, tag each request span with its matched route
	// template (http.route) so span/metric labels stay bounded to templates
	// instead of raw URLs. The middleware runs after mux matches the route.
	if cfg.Otel.Enabled {
		router.Use(otelRouteTagMiddleware)
	}

	if cfg.AuthEnabled {
		router.HandleFunc("/auth", h.Auth.Login).Methods("POST")
	}
	router.Handle("/_mcp", mcpRoute(cfg, h.MCP.Handler())).Methods("GET", "POST")
	router.HandleFunc("/databases", h.Catalog.ListDatabases).Methods("GET")
	router.HandleFunc("/schemas", h.Catalog.ListSchemas).Methods("GET")
	router.HandleFunc("/tables", h.Catalog.ListTables).Methods("GET")

	if h.QueryRegistry != nil && adminStack != nil {
		router.Handle("/_QUERIES/registry", adminRoute(adminStack, h.QueryRegistry.List)).Methods("GET")
		router.Handle("/_QUERIES/registry", adminRoute(adminStack, h.QueryRegistry.Create)).Methods("POST")
		router.Handle("/_QUERIES/registry/{location}/{name}", adminRoute(adminStack, h.QueryRegistry.Get)).Methods("GET")
		router.Handle("/_QUERIES/registry/{location}/{name}", adminRoute(adminStack, h.QueryRegistry.Update)).Methods("PUT")
		router.Handle("/_QUERIES/registry/{location}/{name}", adminRoute(adminStack, h.QueryRegistry.Delete)).Methods("DELETE")
		router.Handle("/_QUERIES/registry/{database}/{location}/{name}", adminRoute(adminStack, h.QueryRegistry.Get)).Methods("GET")
		router.Handle("/_QUERIES/registry/{database}/{location}/{name}", adminRoute(adminStack, h.QueryRegistry.Update)).Methods("PUT")
		router.Handle("/_QUERIES/registry/{database}/{location}/{name}", adminRoute(adminStack, h.QueryRegistry.Delete)).Methods("DELETE")
	}

	router.Handle("/_QUERIES/{queriesLocation}/{script}", queryRoute(queryStack, h.Script.Execute))
	router.Handle("/_QUERIES/{database}/{queriesLocation}/{script}", queryRoute(queryStack, h.Script.Execute))

	if runtime.GOOS != "windows" {
		router.HandleFunc("/_PLUGIN/{file}/{func}", plg.Handler())
	}

	// Studio must be registered before /{database}/{schema} catch-alls.
	router.PathPrefix("/_studio").Handler(studio.Handler(cfg.StudioConf.Enabled))

	router.HandleFunc("/{database}/{schema}", h.Catalog.ListTablesByDatabaseAndSchema).Methods("GET")
	router.HandleFunc("/show/{database}/{schema}/{table}", h.Table.Show).Methods("GET")
	router.HandleFunc("/_health", h.Health.Handler()).Methods("GET")
	router.HandleFunc("/_ready", h.Ready.Handler()).Methods("GET")

	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Select)).Methods("GET")
	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Insert)).Methods("POST")
	router.Handle("/batch/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.BatchInsert)).Methods("POST")
	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Delete)).Methods("DELETE")
	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Update)).Methods("PUT", "PATCH")
}

// otelRouteTagMiddleware annotates the active OpenTelemetry span with the
// matched gorilla/mux path template (http.route). It is a no-op for unmatched
// routes and when no recording span is active.
func otelRouteTagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if route := mux.CurrentRoute(r); route != nil {
			if tmpl, err := route.GetPathTemplate(); err == nil {
				routeAttr := semconv.HTTPRoute(tmpl)
				span := trace.SpanFromContext(r.Context())
				// Semconv HTTP server span name: "{method} {route}".
				span.SetName(r.Method + " " + tmpl)
				span.SetAttributes(routeAttr)
				// Bind the route template to the otelhttp server metrics too.
				if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
					labeler.Add(routeAttr)
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func crudRoute(stack *middlewares.CRUDStack, handler http.HandlerFunc) http.Handler {
	return negroni.New(append(stack.Handlers(), negroni.Wrap(handler))...)
}

func queryRoute(stack *middlewares.QueryStack, handler http.HandlerFunc) http.Handler {
	if stack == nil || len(stack.Handlers()) == 0 {
		return handler
	}
	return negroni.New(append(stack.Handlers(), negroni.Wrap(handler))...)
}

func adminRoute(stack *middlewares.AdminQueryStack, handler http.HandlerFunc) http.Handler {
	return negroni.New(append(stack.Handlers(), negroni.Wrap(handler))...)
}

func mcpRoute(cfg *config.Prest, handler http.HandlerFunc) http.Handler {
	if !cfg.AuthEnabled {
		return handler
	}
	return negroni.New(
		middlewares.AuthMiddleware(middlewares.AuthSettings{
			Enabled:      cfg.AuthEnabled,
			JWTKey:       cfg.JWTKey,
			JWTWhiteList: cfg.JWTWhiteList,
		}),
		negroni.Wrap(handler),
	)
}
