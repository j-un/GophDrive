package handler_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/adapter/dynamo"
	"github.com/jun/gophdrive/backend/internal/handler"
)

func TestExport_ZipsAllNotes(t *testing.T) {
	provider := dynamo.NewProvider(nil)
	noteH := handler.NewNoteHandler(provider, "test-secret")
	exportH := handler.NewExportHandler(provider, "test-secret")
	ctx := context.Background()

	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"a.md","content":"alpha"}`))
	noteH.CreateNote(ctx, makeRequest("POST", "/notes", `{"name":"b.md","content":"beta"}`))

	resp, err := exportH.Export(ctx, makeRequest("GET", "/export", ""))
	if err != nil {
		t.Fatalf("Export err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d (%s)", resp.StatusCode, resp.Body)
	}
	if !resp.IsBase64Encoded {
		t.Fatal("expected IsBase64Encoded=true for binary ZIP body")
	}
	if got := resp.Headers["Content-Type"]; got != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", got)
	}
	if got := resp.Headers["Content-Disposition"]; got == "" {
		t.Error("missing Content-Disposition header")
	}

	raw, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		t.Fatalf("decode body: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %q: %v", f.Name, err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(body)
	}
	if got["a.md"] != "alpha" {
		t.Errorf("a.md content = %q, want alpha", got["a.md"])
	}
	if got["b.md"] != "beta" {
		t.Errorf("b.md content = %q, want beta", got["b.md"])
	}
}

func TestExport_Unauthorized(t *testing.T) {
	exportH := handler.NewExportHandler(dynamo.NewProvider(nil), "test-secret")
	resp, _ := exportH.Export(context.Background(), events.APIGatewayProxyRequest{})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestExport_EmptyArchiveStillSucceeds(t *testing.T) {
	exportH := handler.NewExportHandler(dynamo.NewProvider(nil), "test-secret")
	resp, err := exportH.Export(context.Background(), makeRequest("GET", "/export", ""))
	if err != nil {
		t.Fatalf("Export err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 even when user has no notes, got %d (%s)", resp.StatusCode, resp.Body)
	}
	raw, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	if len(zr.File) != 0 {
		t.Errorf("expected empty zip, got %d files", len(zr.File))
	}
}
