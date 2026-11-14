package httpapi

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	maxNIDImageBytes       = 4 << 20 // 4 MB, matches Filament maxSize(4096)
	maxTradeLicenseBytes   = 5 << 20 // 5 MB, matches Filament maxSize(5120)
	maxLivePhotoBytes      = 5 << 20
	livePhotoDataURIPrefix = "data:image/"
)

var errFileTooLarge = fmt.Errorf("file is larger than the allowed size")

func (server *Server) storageRoot() string {
	if server.kycService != nil && server.kycService.StorageRoot() != "" {
		return server.kycService.StorageRoot()
	}
	if root := os.Getenv("STORAGE_PATH"); root != "" {
		return root
	}
	if root := os.Getenv("KYC_STORAGE_PATH"); root != "" {
		return root
	}

	return "./storage/app/private"
}

func imageExtensionFor(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	case "image/bmp":
		return ".bmp", true
	}

	return "", false
}

// saveUploadedFile stores an uploaded image (or PDF for trade licences) on the
// private disk, mirroring Filament FileUpload with directory(...) + visibility('private').
// It returns the relative path stored in the users table.
func (server *Server) saveUploadedFile(header *multipart.FileHeader, dir string, maxBytes int64, allowPDF bool) (string, error) {
	if header == nil {
		return "", fmt.Errorf("file is required")
	}

	src, err := header.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		return "", errFileTooLarge
	}

	contentType := http.DetectContentType(data)
	ext, ok := imageExtensionFor(contentType)
	if allowPDF && contentType == "application/pdf" {
		ext, ok = ".pdf", true
	}
	if !ok {
		if allowPDF {
			return "", fmt.Errorf("file must be a JPG, PNG, WEBP, GIF, BMP or PDF")
		}

		return "", fmt.Errorf("file must be a JPG, PNG, WEBP, GIF or BMP image")
	}

	name := uuid.NewString() + ext
	targetDir := filepath.Join(server.storageRoot(), dir)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(targetDir, name), data, 0o640); err != nil {
		return "", err
	}

	return dir + "/" + name, nil
}

// saveLivePhoto decodes the base64 data URL produced by the browser camera and
// stores it exactly like the old LivePhotoCapture component (live_photos/live_<uuid>.jpg).
func (server *Server) saveLivePhoto(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("live photo is required")
	}
	if !strings.HasPrefix(strings.ToLower(raw), livePhotoDataURIPrefix) {
		return "", fmt.Errorf("live photo must be captured with the device camera")
	}

	comma := strings.Index(raw, ",")
	if comma < 0 || !strings.Contains(strings.ToLower(raw[:comma]), ";base64") {
		return "", fmt.Errorf("live photo data is invalid")
	}

	data, err := base64.StdEncoding.DecodeString(raw[comma+1:])
	if err != nil {
		return "", fmt.Errorf("live photo data is invalid")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("live photo is empty")
	}
	if len(data) > maxLivePhotoBytes {
		return "", errFileTooLarge
	}

	contentType := http.DetectContentType(data)
	ext, ok := imageExtensionFor(contentType)
	if !ok {
		return "", fmt.Errorf("live photo must be a JPG, PNG, WEBP, GIF or BMP image")
	}

	name := "live_" + uuid.NewString() + ext
	targetDir := filepath.Join(server.storageRoot(), "live_photos")
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(targetDir, name), data, 0o640); err != nil {
		return "", err
	}

	return "live_photos/" + name, nil
}

// removeStoredFiles is used to roll back uploads when the database write fails.
func (server *Server) removeStoredFiles(paths ...string) {
	root := server.storageRoot()
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
	}
}
