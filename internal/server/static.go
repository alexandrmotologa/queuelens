package server

import (
	"net/http"
	"strings"

	"github.com/alexandrmotologa/queuelens/web"
)

// RegisterStaticHandlers binds SPA static file serving to the ServeMux.
func RegisterStaticHandlers(mux *http.ServeMux) {
	fileServer := http.FileServer(web.GetFileSystem())

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Do not intercept API requests
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}

		// Try to serve static file from embedded dist
		f, err := web.GetFileSystem().Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA client routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
