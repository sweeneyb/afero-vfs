package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/afero"
)

var (
	// Store per-user filesystems
	userFS   = make(map[string]afero.Fs)
	fsLock   sync.RWMutex
	fileRefs = make(map[string]FileMeta) // GUID -> metadata
	refLock  sync.RWMutex
)

type FileMeta struct {
	User     string
	Filename string
}

// Get or create a user's filesystem
func getUserFS(username string) afero.Fs {
	fsLock.Lock()
	defer fsLock.Unlock()
	if fs, exists := userFS[username]; exists {
		return fs
	}
	memFs := afero.NewMemMapFs()
	userFS[username] = memFs
	return memFs
}

// Upload handler
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	user := r.FormValue("user")
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content into memory
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Write to virtual filesystem
	fs := getUserFS(user)
	filename := header.Filename
	if err := afero.WriteFile(fs, filename, buf.Bytes(), 0644); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	// Generate GUID
	guid := uuid.New().String()

	// Store file reference
	refLock.Lock()
	fileRefs[guid] = FileMeta{User: user, Filename: filename}
	refLock.Unlock()

	w.Write([]byte(fmt.Sprintf("Uploaded with GUID: %s", guid)))
}

// Copy a file from one user to another using source path
func copyByPathHandler(w http.ResponseWriter, r *http.Request) {
	fromUser := r.URL.Query().Get("from_user")
	toUser := r.URL.Query().Get("to_user")
	srcPath := r.URL.Query().Get("src_path")
	subfolder := r.URL.Query().Get("subfolder")

	if fromUser == "" || toUser == "" || srcPath == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	srcFS := getUserFS(fromUser)
	destFS := getUserFS(toUser)

	data, err := afero.ReadFile(srcFS, srcPath)
	if err != nil {
		http.Error(w, "Source file not found", http.StatusNotFound)
		return
	}

	destPath := srcPath
	if subfolder != "" {
		destPath = fmt.Sprintf("%s/%s", subfolder, afero.FilePathSeparator+srcPath)
	}

	// Ensure destination subfolder exists
	if subfolder != "" {
		aferoFs := afero.Afero{Fs: destFS}
		if err := aferoFs.MkdirAll(subfolder, 0755); err != nil {
			http.Error(w, "Failed to create subfolder", http.StatusInternalServerError)
			return
		}
	}

	if err := afero.WriteFile(destFS, destPath, data, 0644); err != nil {
		http.Error(w, "Failed to write to destination", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("Copied '%s' to user '%s' at '%s'", srcPath, toUser, destPath)))
}


// Copy handler
func copyHandler(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("guid")
	destUser := r.URL.Query().Get("to_user")
	subfolder := r.URL.Query().Get("subfolder")

	refLock.RLock()
	meta, exists := fileRefs[guid]
	refLock.RUnlock()

	if !exists {
		http.Error(w, "Invalid GUID", http.StatusNotFound)
		return
	}

	srcFS := getUserFS(meta.User)
	destFS := getUserFS(destUser)

	// Read from source
	data, err := afero.ReadFile(srcFS, meta.Filename)
	if err != nil {
		http.Error(w, "Failed to read source file", http.StatusInternalServerError)
		return
	}

	// Create subfolder in dest if needed
	fullPath := fmt.Sprintf("%s/%s", subfolder, meta.Filename)
	if err := afero.WriteFile(destFS, fullPath, data, 0644); err != nil {
		http.Error(w, "Failed to write to destination", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("File copied to user '%s' at '%s'", destUser, fullPath)))
}

// List directory contents
func listHandler(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	if user == "" {
		http.Error(w, "Missing user", http.StatusBadRequest)
		return
	}

	fs := getUserFS(user)
	var buffer bytes.Buffer

	err := afero.Walk(fs, ".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		buffer.WriteString(fmt.Sprintf("%s\n", path))
		return nil
	})
	if err != nil {
		http.Error(w, "Failed to walk filesystem", http.StatusInternalServerError)
		return
	}

	w.Write(buffer.Bytes())
}

// Download a file by path
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	path := r.URL.Query().Get("path")

	if user == "" || path == "" {
		http.Error(w, "Missing user or path", http.StatusBadRequest)
		return
	}

	fs := getUserFS(user)
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", path))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
}


func main() {
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/copy", copyHandler)
	http.HandleFunc("/copy_by_path", copyByPathHandler)
	http.HandleFunc("/list", listHandler) 
	http.HandleFunc("/download", downloadHandler)
	log.Println("Server running at :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
