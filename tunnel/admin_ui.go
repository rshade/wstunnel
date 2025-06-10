// Copyright (c) 2014 RightScale, Inc. - see LICENSE

package tunnel

import (
	_ "embed"
	"net/http"
	"strings"
)

// Embed the admin UI HTML file
//
//go:embed admin.html
var adminUIHTML string

// HandleAdminUI serves the admin web interface
func (as *AdminService) HandleAdminUI(w http.ResponseWriter, r *http.Request) {
	safeW := &safeResponseWriter{ResponseWriter: w}

	if r.Method != "GET" {
		safeError(safeW, "Only GET requests are supported", http.StatusMethodNotAllowed)
		return
	}

	// Set appropriate headers
	safeW.Header().Set("Content-Type", "text/html; charset=utf-8")
	safeW.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	safeW.Header().Set("Pragma", "no-cache")
	safeW.Header().Set("Expires", "0")

	// Serve the embedded HTML
	if _, err := safeW.Write([]byte(adminUIHTML)); err != nil {
		as.log.Error("Failed to write admin UI response", "err", err)
	}
}

// HandleAdminUIRedirect redirects /admin to /admin/ui for convenience
func (as *AdminService) HandleAdminUIRedirect(w http.ResponseWriter, r *http.Request) {
	// Get the base path from the current request
	basePath := ""
	if r.URL.Path != "/admin" {
		// Extract base path by removing /admin from the end
		fullPath := r.URL.Path
		if strings.HasSuffix(fullPath, "/admin") {
			basePath = strings.TrimSuffix(fullPath, "/admin")
		}
	}

	// Redirect to the UI endpoint
	redirectURL := basePath + "/admin/ui"
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}
