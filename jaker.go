package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const defaultPort = 8080
const defaultStorageDir = "./storage"
const maxUploadSize = 5 * 1024 * 1024 // 5 MB

var storageDir string // Define storageDir as a global variable

func main() {
	// Parse command-line flags
	var port int
	var certFile, keyFile, storageDirFlag string
	var daemonize bool

	flag.IntVar(&port, "port", defaultPort, "Port number to listen on")
	flag.StringVar(&storageDirFlag, "storage-dir", defaultStorageDir, "Storage directory for uploaded files")
	flag.StringVar(&certFile, "cert-file", "", "Path to SSL certificate file")
	flag.StringVar(&keyFile, "key-file", "", "Path to SSL private key file")
	flag.BoolVar(&daemonize, "daemon", false, "Run as a daemon")
	flag.Parse()

	// Use the command-line flag value for storageDir, if provided
	if storageDirFlag != "" {
		storageDir = storageDirFlag
	}

	// Check if the storage directory exists, and if not, ask the user if they want to create it
	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		var answer string
		fmt.Printf("Storage directory '%s' does not exist. Do you want to create it? (y/n): ", storageDir)
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) == "y" {
			err = os.MkdirAll(storageDir, 0755)
			if err != nil {
				log.Fatalf("Error creating the storage directory: %v", err)
			}
		} else {
			log.Fatalln("Storage directory not found. Exiting.")
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
		} else {
			fmt.Println("Running in the foreground...")
		}

		err := server.ListenAndServeTLS(certFile, keyFile)
		if err != nil {
			log.Fatalf("Error starting the server with SSL/TLS: %v", err)
		}
	} else {
		fmt.Printf("Starting server on port %d...\n", port)
		if daemonize {
			// Daemonize the server by forking a new process
			// and detach the child process from the terminal
			daemonizeServer()
		} else {
			fmt.Println("Running in the foreground...")
		}

		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Fatalf("Error starting the server: %v", err)
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
		log.Printf("Error parsing form: %v", err)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		log.Printf("Error retrieving file: %v", err)
		return
	}
	defer file.Close()

	bucket := r.FormValue("bucket")
	path := r.FormValue("path")

	fileName := filepath.Base(path)
	// Create the bucket path if it doesn't exist
	bucketPath := filepath.Join(storageDir, bucket)
	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		if err := os.MkdirAll(bucketPath, 0755); err != nil {
			http.Error(w, "Error creating bucket path", http.StatusInternalServerError)
			log.Printf("Error creating bucket path: %v", err)
			return
		}
	}

	dst, err := os.Create(filepath.Join(bucketPath, fileName))
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		log.Printf("Error creating file: %v", err)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		log.Printf("Error copying file: %v", err)
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

	filePath := filepath.Join(storageDir, bucket, objectName)
	http.ServeFile(w, r, filePath)
}
