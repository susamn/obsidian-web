package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// spaHandler implements the http.Handler interface and serves a single page application.
type spaHandler struct {
	staticPath string
	indexPath  string
}

// ServeHTTP serves the single page application.
func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// if the request is for an API endpoint, let the default mux handle it
	if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/swagger") {
		http.NotFound(w, r)
		return
	}

	// get the absolute path to prevent directory traversal
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// prepend the path with the path to the static directory
	path = filepath.Join(h.staticPath, path)

	// check whether a file exists at the given path
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		// file does not exist, serve index.html
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil {
		// if we got an error (that wasn't that the file doesn't exist) stating the
		// file, return a 500 internal server error and stop
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// otherwise, use http.FileServer to serve the static file
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}
