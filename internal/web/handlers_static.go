package web

import (
	"net/http"
	"path/filepath"
	"strings"
)

// handleIndex serves the main web UI
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// For now, serve a simple HTML page
	// TODO: Replace with actual web UI
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(indexHTML))
}

// handleStatic serves static files
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Remove /static/ prefix
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// Security: prevent directory traversal
	if strings.Contains(path, "..") {
		writeError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	// Get static directory from config
	staticDir := s.config.Server.StaticDir
	if staticDir == "" {
		staticDir = "./static"
	}

	// Build full path
	fullPath := filepath.Join(staticDir, path)

	// Serve file
	http.ServeFile(w, r, fullPath)
}

// indexHTML is a simple placeholder HTML page
const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Obsidian Web</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 { color: #333; }
        .endpoints {
            background: #f5f5f5;
            padding: 20px;
            border-radius: 5px;
            margin: 20px 0;
        }
        code {
            background: #e0e0e0;
            padding: 2px 6px;
            border-radius: 3px;
        }
    </style>
</head>
<body>
    <h1>Obsidian Web API</h1>
    <p>Welcome to the Obsidian Web API server.</p>

    <div class="endpoints">
        <h2>Available Endpoints:</h2>
        <ul>
            <li><code>GET /api/v1/health</code> - Health check</li>
            <li><code>GET /api/v1/vaults</code> - List all vaults</li>
            <li><code>GET /api/v1/vaults/:id</code> - Get vault info</li>
            <li><code>GET /api/v1/metrics/:vault</code> - Vault metrics</li>
            <li><code>POST /api/v1/search/:vault</code> - Search vault</li>
            <li><code>GET /api/v1/files/:vault/:path</code> - Get file</li>
        </ul>
    </div>
</body>
</html>`
