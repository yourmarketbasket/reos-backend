package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/reos/api/internal/store"
)

type UploadsHandler struct {
	Store *store.Store
}

func NewUploadsHandler(s *store.Store) *UploadsHandler {
	return &UploadsHandler{Store: s}
}

func (h *UploadsHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate authorization
	_, err := getUserIdFromAuthHeader(r, h.Store)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Limit request size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)
	err = r.ParseMultipartForm(10 * 1024 * 1024)
	if err != nil {
		http.Error(w, "File is too large (maximum 10MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("image")
	}
	if err != nil {
		http.Error(w, "file or image form parameter is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Check file extension / content-type
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "Invalid content type: only image files are allowed", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" {
		http.Error(w, "Invalid file extension: only jpg, jpeg, png, webp, and gif are allowed", http.StatusBadRequest)
		return
	}

	// Build multipart upload payload to Nisoko
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("file", header.Filename)
	if err != nil {
		http.Error(w, "Failed to create multipart form", http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(part, file)
	if err != nil {
		http.Error(w, "Failed to copy file to multipart form", http.StatusInternalServerError)
		return
	}

	err = writer.Close()
	if err != nil {
		http.Error(w, "Failed to close multipart writer", http.StatusInternalServerError)
		return
	}

	// Fetch credentials from env configurations
	container := os.Getenv("NISOKO_CONTAINER")
	apiKey := os.Getenv("NISOKO_API_KEY")
	if container == "" || apiKey == "" {
		container = "reos-assets"
		apiKey = "nsk_live_e465b70924be48d3f4e5281d42fa8aad3bcd7077e7010b9c"
	}

	uploadURL := fmt.Sprintf("https://storage.nisoko.co.ke/api/v1/storage/containers/%s/upload", container)
	req, err := http.NewRequest("POST", uploadURL, &requestBody)
	if err != nil {
		http.Error(w, "Failed to create request to Nisoko", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to upload to Nisoko: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Nisoko upload failed: status %s, body: %s", resp.Status, string(respBody)), http.StatusBadGateway)
		return
	}

	// Parse Nisoko Response
	var nisokoResp struct {
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&nisokoResp); err != nil {
		http.Error(w, "Failed to parse Nisoko response", http.StatusInternalServerError)
		return
	}

	if nisokoResp.DownloadURL == "" {
		http.Error(w, "Nisoko returned an empty download URL", http.StatusBadGateway)
		return
	}

	// Return URL to frontend client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": nisokoResp.DownloadURL,
	})
}
