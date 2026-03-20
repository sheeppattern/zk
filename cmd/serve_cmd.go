package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/sheeppattern/zk/internal/store"
)

//go:embed web
var webContent embed.FS

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web GUI for browsing and editing the knowledge graph",
	Long:  "Launch a local web server that provides an Obsidian-style interface for exploring notes, editing content, and visualizing the knowledge graph.",
	Example: `  zk serve
  zk serve --addr :3000
  zk serve --addr 127.0.0.1:8080`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().String("addr", ":8080", "listen address (host:port)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	storePath := getStorePath(cmd)

	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		return fmt.Errorf("store not found at %s; run 'zk init' first", storePath)
	}

	s := store.NewStore(storePath)

	mux := http.NewServeMux()

	h := &serveHandler{store: s}
	mux.HandleFunc("/api/projects", h.handleProjects)
	mux.HandleFunc("/api/notes", h.handleNotes)
	mux.HandleFunc("/api/note", h.handleNote)
	mux.HandleFunc("/api/search", h.handleSearch)
	mux.HandleFunc("/api/all-data", h.handleAllData)
	mux.HandleFunc("/api/tag", h.handleTag)
	mux.HandleFunc("/api/link", h.handleLink)

	webFS, err := fs.Sub(webContent, "web")
	if err != nil {
		return fmt.Errorf("embed filesystem: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	handler := maxBodyMiddleware(corsMiddleware(mux))

	if strings.HasPrefix(addr, ":") {
		statusf("zk web GUI at http://localhost%s", addr)
	} else {
		statusf("zk web GUI at http://%s", addr)
	}
	statusf("store: %s", storePath)
	statusf("press Ctrl+C to stop")

	return http.ListenAndServe(addr, handler)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only reflect CORS headers for requests with an Origin header (same-origin
		// requests from the embedded UI don't send Origin, so no header is needed).
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// maxBodyMiddleware limits request body size to prevent DoS.
func maxBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		next.ServeHTTP(w, r)
	})
}
