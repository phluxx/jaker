package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const defaultPort = 8080
const defaultStorageDir = "./storage"
const maxUploadSize = 5 * 1024 * 1024 // 5 MB

func main() {
	// Parse command-line flags
	var port int
	var storageDir, certFile, keyFile string
	var daemonize bool

	flag.IntVar(&port, "port", defaultPort, "Port number to listen on")
	flag.StringVar(&storageDir, "storage-dir", defaultStorageDir, "Storage directory for uploaded files")
	flag.StringVar(&certFile, "cert-file", "", "Path to SSL certificate file")
	flag.StringVar(&keyFile, "key-file", "", "Path to SSL private key file")
	flag.BoolVar(&daemonize, "daemon", false, "Run as a daemon")
	flag.Parse()

	// Check if the storage directory exists, and if not, ask the user if they want to create it
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		var answer string
		fmt.Printf("Storage directory '%s' does not exist. Do you want to create it? (y/n): ", storageDir)
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) == "y" {
			err = os.MkdirAll(storageDir, 0755)
			if err != nil {
				fmt.Println("Error creating the storage directory:", err)
				os.Exit(1)
			}
		} else {
			os.Exit(1)
		}
	}

	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/", serveImageHandler)

	addr := fmt.Sprintf(":%d", port)

	if certFile != "" && keyFile != "" {
		fmt.Printf("Starting server on port %d with SSL/TLS enabled...\n", port)
		// Create TLS configuration
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		server := &http.Server{
			Addr:      addr,
			Handler:   nil, // Use the default HTTP server mux
			TLSConfig: tlsConfig,
		}

		if daemonize {
			// Daemonize the server by forking a new process
			// and detach the child process from the terminal
			daemonizeServer()
		}

		err := server.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			fmt.Println("Error starting the server:", err)
		}
	} else {
		fmt.Printf("Starting server on port %d...\n", port)
		if daemonize {
			// Daemonize the server by forking a new process
			// and detach the child process from the terminal
			daemonizeServer()
		}

		err := http.ListenAndServe(addr, nil)
		if err != nil {
			fmt.Println("Error starting the server:", err)
		}
	}
}

func daemonizeServer() {
	// Fork a new process
	ret, _, _ := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	if ret == 0 {
		// Child process, continue execution
		return
	}

	// Parent process, exit
	os.Exit(0)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	bucket := r.FormValue("bucket")
	path := r.FormValue("path")

	fileName := filepath.Base(path)
	dst, err := os.Create(filepath.Join(defaultStorageDir, bucket, fileName))
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "File uploaded successfully.")
}

func serveImageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path[1:]
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	bucket := parts[0]
	objectName := parts[1]

	filePath := filepath.Join(defaultStorageDir, bucket, objectName)
	http.ServeFile(w, r, filePath)
}
