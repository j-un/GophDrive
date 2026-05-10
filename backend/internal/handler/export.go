package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter"
)

// ExportHandler streams every note owned by the caller as a single ZIP.
// Returned through API Gateway as base64 (IsBase64Encoded=true) — the only
// supported way to deliver binary bodies through the v1 proxy integration.
type ExportHandler struct {
	storageProvider adapter.StorageProvider
	jwtSecret       string
}

func NewExportHandler(provider adapter.StorageProvider, jwtSecret string) *ExportHandler {
	return &ExportHandler{storageProvider: provider, jwtSecret: jwtSecret}
}

// Export handles GET /export.
func (h *ExportHandler) Export(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	claims, err := GetSessionClaims(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "unauthorized"}, nil
	}

	storage, err := h.storageProvider.GetAdapter(ctx, claims.UserID, claims.BaseFolderID)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "failed to get storage adapter"}, nil
	}

	entries, err := storage.Export(ctx)
	if err != nil {
		fmt.Printf("Export error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "failed to export notes"}, nil
	}

	zipped, err := writeZip(entries)
	if err != nil {
		fmt.Printf("zip build error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "failed to build archive"}, nil
	}

	filename := fmt.Sprintf("gophdrive-export-%s.zip", time.Now().UTC().Format("20060102-150405"))
	return events.APIGatewayProxyResponse{
		StatusCode:      http.StatusOK,
		Body:            base64.StdEncoding.EncodeToString(zipped),
		IsBase64Encoded: true,
		Headers: map[string]string{
			"Content-Type":        "application/zip",
			"Content-Disposition": fmt.Sprintf("attachment; filename=%q", filename),
		},
	}, nil
}

// writeZip writes each export entry into a deterministic zip archive. Path
// collisions are de-duplicated with a numeric suffix so two notes that
// happen to share a name (different folders shouldn't, but inline-renamed
// rows might) don't silently overwrite each other inside the archive.
func writeZip(entries []adapter.ExportEntry) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	used := make(map[string]int, len(entries))
	for _, e := range entries {
		path := e.Path
		if n, seen := used[path]; seen {
			used[path] = n + 1
			path = uniquePath(e.Path, n+1)
		}
		used[path]++

		hdr := &zip.FileHeader{
			Name:     path,
			Method:   zip.Deflate,
			Modified: e.ModifiedTime,
		}
		fw, err := w.CreateHeader(hdr)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(e.Content); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func uniquePath(path string, n int) string {
	const ext = ".md"
	if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
		base := path[:len(path)-len(ext)]
		return fmt.Sprintf("%s (%d)%s", base, n, ext)
	}
	return fmt.Sprintf("%s (%d)", path, n)
}
