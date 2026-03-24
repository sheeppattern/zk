package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed web
var webContent embed.FS

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web GUI for browsing and editing the knowledge graph",
	Long:  "Launch a local web server that provides an interface for exploring memos, editing content, and visualizing the knowledge graph.",
	Example: `  zk serve
  zk serve --addr :3000
  zk serve --addr 127.0.0.1:8080`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().String("addr", "127.0.0.1:8080", "listen address (host:port)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	addr, _ := cmd.Flags().GetString("addr")

	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	defer s.Close()

	mux := http.NewServeMux()

	h := &serveHandler{store: s}
	mux.HandleFunc("/api/notes", h.handleNotes)
	mux.HandleFunc("/api/memos", h.handleMemos)
	mux.HandleFunc("/api/memo", h.handleMemo)
	mux.HandleFunc("/api/search", h.handleSearch)
	mux.HandleFunc("/api/all-data", h.handleAllData)
	mux.HandleFunc("/api/tag", h.handleTag)
	mux.HandleFunc("/api/link", h.handleLink)

	webFS, err := fs.Sub(webContent, "web")
	if err != nil {
		return fmt.Errorf("embed filesystem: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	handler := maxBodyMiddleware(mux)

	if strings.HasPrefix(addr, ":") {
		statusf("zk web GUI at http://localhost%s", addr)
	} else {
		statusf("zk web GUI at http://%s", addr)
	}
	statusf("press Ctrl+C to stop")

	return http.ListenAndServe(addr, handler)
}

// maxBodyMiddleware limits request body size to prevent DoS.
func maxBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		next.ServeHTTP(w, r)
	})
}
