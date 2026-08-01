package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)
func apiCalendarGet(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if month < 1 || month > 12 {
		month = int(time.Now().Month())
	}
	if year == 0 {
		year = time.Now().Year()
	}

	counts, err := getWordCounts(db, year, month)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	goal, err := getGoal(db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	goalInt, _ := strconv.Atoi(goal)

	days := make(map[string]bool)
	for _, c := range counts {
		if c.Count >= goalInt {
			days[c.Date] = true
		}
	}
	streak, err := getStreak(db, goalInt)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"year":   year,
		"month":  month,
		"days":   days,
		"streak": streak,
	})
}



type FileNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	IsDir    bool       `json:"isDir"`
	Children []FileNode `json:"children,omitempty"`
}

func apiTree(w http.ResponseWriter, r *http.Request, root string) {
	tree, err := buildTree(root, root)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tree)
}

func buildTree(baseDir, currentDir string) ([]FileNode, error) {
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return nil, err
	}
	var nodes []FileNode
	for _, entry := range entries {
		if entry.Name()[0] == '.' {
			continue
		}
		fullPath := filepath.Join(currentDir, entry.Name())
		relPath, _ := filepath.Rel(baseDir, fullPath)
		if entry.IsDir() {
			children, err := buildTree(baseDir, fullPath)
			if err != nil {
				continue
			}
			if len(children) == 0 {
				continue
			}
			nodes = append(nodes, FileNode{
				Name:     entry.Name(),
				Path:     relPath,
				IsDir:    true,
				Children: children,
			})
		} else if strings.HasSuffix(entry.Name(), ".md") || isImageFile(entry.Name()) {
			nodes = append(nodes, FileNode{
				Name:  entry.Name(),
				Path:  relPath,
				IsDir: false,
			})
		}
	}
	return nodes, nil
}

func isImageFile(name string) bool {
	ext := strings.ToLower(name)
	for _, e := range []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp"} {
		if strings.HasSuffix(ext, e) {
			return true
		}
	}
	return false
}

func apiFileGet(w http.ResponseWriter, r *http.Request, root string) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "missing path", 400)
		return
	}
	fullPath := filepath.Join(root, filepath.Clean(relPath))
	absRoot, _ := filepath.Abs(root)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) {
		http.Error(w, "forbidden", 403)
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"content": string(data)})
}

func apiFilePut(w http.ResponseWriter, r *http.Request, root string, db *sql.DB) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "missing path", 400)
		return
	}
	fullPath := filepath.Join(root, filepath.Clean(relPath))
	absRoot, _ := filepath.Abs(root)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) {
		http.Error(w, "forbidden", 403)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", 400)
		return
	}
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Record word count for the daily calendar widget
	if strings.HasSuffix(fullPath, ".md") && db != nil {
		text := strings.TrimSpace(req.Content)
		count := 0
		if text != "" {
			count = len(strings.Fields(text))
		}
		recordWordCount(db, count)
	}
	w.WriteHeader(200)
}

func apiFilePost(w http.ResponseWriter, r *http.Request, root string) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "missing path", 400)
		return
	}
	fullPath := filepath.Join(root, filepath.Clean(relPath))
	absRoot, _ := filepath.Abs(root)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) {
		http.Error(w, "forbidden", 403)
		return
	}
	if !strings.HasSuffix(fullPath, ".md") {
		fullPath += ".md"
	}
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := os.WriteFile(fullPath, []byte(""), 0644); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(201)
}

func apiFileDelete(w http.ResponseWriter, r *http.Request, root string) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "missing path", 400)
		return
	}
	fullPath := filepath.Join(root, filepath.Clean(relPath))
	absRoot, _ := filepath.Abs(root)
	absPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absPath, absRoot) {
		http.Error(w, "forbidden", 403)
		return
	}
	if err := os.Remove(fullPath); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}

func apiUpload(w http.ResponseWriter, r *http.Request, root string) {
	dirRel := r.URL.Query().Get("dir")
	if dirRel == "" {
		dirRel = "."
	}
	uploadDir := filepath.Join(root, filepath.Clean(dirRel))
	absRoot, _ := filepath.Abs(root)
	absDir, _ := filepath.Abs(uploadDir)
	if !strings.HasPrefix(absDir, absRoot) {
		http.Error(w, "forbidden", 403)
		return
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10MB
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer file.Close()

	filename := header.Filename
	dstPath := filepath.Join(uploadDir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	relRef, _ := filepath.Rel(root, dstPath)
	// Percent-encode the URL so filenames with spaces etc. are valid markdown
	encRef := (&url.URL{Path: filepath.ToSlash(relRef)}).String()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"markdown": "![" + strings.TrimSuffix(filename, filepath.Ext(filename)) + "](" + encRef + ")",
	})
}

func apiGoalGet(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	goal, err := getGoal(db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"goal": goal})
}

func apiGoalPut(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var req map[string]string
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	goal := req["goal"]
	if goal == "" {
		http.Error(w, "missing goal", 400)
		return
	}
	if err := setGoal(db, goal); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
}
