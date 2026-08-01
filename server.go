package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
)

func openBrowser(url string) {
	switch runtime.GOOS {
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "darwin":
		exec.Command("open", url).Start()
	case "windows":
		exec.Command("cmd", "/c", "start", url).Start()
	}
}

func findFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func startServer(root string, db *sql.DB, openFile string, sidebarOpen bool) error {
	port, err := findFreePort()
	if err != nil {
		return fmt.Errorf("finding port: %w", err)
	}

	// Strip "web/" prefix from embedded FS
	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("stripping web prefix: %w", err)
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		apiTree(w, r, root)
	})
	mux.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			apiFileGet(w, r, root)
		case "PUT":
			apiFilePut(w, r, root)
		case "POST":
			apiFilePost(w, r, root)
		case "DELETE":
			apiFileDelete(w, r, root)
		default:
			http.Error(w, "method not allowed", 405)
		}
	})
	mux.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		apiUpload(w, r, root)
	})
	mux.HandleFunc("/api/goal", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			apiGoalGet(w, r, db)
		case "PUT":
			apiGoalPut(w, r, db)
		default:
			http.Error(w, "method not allowed", 405)
		}
	})

	// Serve embedded web assets (index.html, style.css, app.js)
	webServer := http.FileServer(http.FS(webSub))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try embedded web assets first
		path := r.URL.Path
		if path == "/" || path == "/index.html" || path == "/style.css" || path == "/app.js" || path == "/marked.min.js" || path == "/medium-zoom.min.js" {
			webServer.ServeHTTP(w, r)
			return
		}
		// Fallback: serve from working directory (for images referenced in markdown)
		fileServer := http.FileServer(http.Dir(root))
		fileServer.ServeHTTP(w, r)
	}))

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	if openFile != "" {
		url += fmt.Sprintf("?file=%s&sidebar=hidden", openFile)
	} else if sidebarOpen {
		url += "?sidebar=open"
	}

	fmt.Printf("writr: serving on %s\n", url)
	openBrowser(url)

	return http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), mux)
}
