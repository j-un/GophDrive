package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/auth"
	xoauth2 "golang.org/x/oauth2"
	"google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// AuthHandler handles authentication requests.
type AuthHandler struct {
	authService     *auth.AuthService
	storageProvider adapter.StorageProvider
	jwtSecret       string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(s *auth.AuthService, sp adapter.StorageProvider, jwtSecret string) *AuthHandler {
	return &AuthHandler{authService: s, storageProvider: sp, jwtSecret: jwtSecret}
}

// Login initiates the Google OAuth2 flow.
func (h *AuthHandler) Login(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// TODO: Generate a secure random state and store it in a cookie to prevent CSRF
	state := "random-state"
	url := h.authService.GenerateAuthURL(state)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusFound,
		Headers: map[string]string{
			"Location": url,
		},
	}, nil
}

// Callback handles the OAuth2 callback from Google.
func (h *AuthHandler) Callback(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	code := req.QueryStringParameters["code"]
	if code == "" {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Missing code"}, nil
	}

	// Exchange code for token
	token, err := h.authService.ExchangeCode(ctx, code)
	if err != nil {
		fmt.Printf("ExchangeCode error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to exchange code"}, nil
	}

	// Get User Info from Google
	oauth2Service, err := oauth2.NewService(ctx, option.WithTokenSource(h.authService.Config().TokenSource(ctx, token)))
	if err != nil {
		fmt.Printf("NewService error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to create oauth2 service"}, nil
	}

	userinfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		fmt.Printf("Userinfo error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to get user info"}, nil
	}

	// Save Token (Refresh Token) to DynamoDB
	// Note: We use userinfo.Id (Google Subject ID) as UserID.
	userID := userinfo.Id

	err = h.authService.SaveToken(ctx, userID, token)
	if err != nil {
		fmt.Printf("SaveToken error: %v\n", err)
		// Proceed even if saving refresh token failed (e.g. no refresh token returned on subsequent login)
		// Ideally we should warn or handle this better.
	}

	// Generate JWT Session Token
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": userinfo.Email,
		"name":  userinfo.Name,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := jwtToken.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to sign token"}, nil
	}

	// Redirect to Frontend with success
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// Determine Cookie Settings based on Environment
	sameSite := "Lax"
	if os.Getenv("DEV_MODE") != "true" {
		// Production (AWS): Frontend (CloudFront) and API (Gateway) share domain via CloudFront but
		// aggressive caching or strict browser policies might require None for reliable auth across reloads.
		sameSite = "None"
	}

	// Set secure httpOnly cookie
	cookie := fmt.Sprintf("session_token=%s; HttpOnly; Path=/; Max-Age=86400; SameSite=%s; Secure", signedToken, sameSite)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusFound,
		Headers: map[string]string{
			"Location": fmt.Sprintf("%s/?success=true", frontendURL),
		},
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {cookie},
		},
	}, nil
}

// ListDriveFolders lists the root folders in Google Drive (or Memory).
func (h *AuthHandler) ListDriveFolders(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 1. Validate Session
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	// 2. Get Adapter
	adapter, err := h.storageProvider.GetAdapter(ctx, userID)
	if err != nil {
		fmt.Printf("GetAdapter error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to get storage adapter"}, nil
	}

	// 3. List Root Folders
	folders, err := adapter.ListRootFolders(ctx)
	if err != nil {
		fmt.Printf("ListRootFolders error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to list folders"}, nil
	}

	body, _ := json.Marshal(folders)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// GetUser returns the current user's profile.
func (h *AuthHandler) GetUser(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 1. Validate Session
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	// 2. Get User Token (Profile)
	token, err := h.authService.GetUserToken(ctx, userID)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to get user profile"}, nil
	}

	// 3. Return Profile
	profile := map[string]string{
		"id":             token.UserID,
		"base_folder_id": token.BaseFolderID,
	}

	body, _ := json.Marshal(profile)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// UpdateUser updates user settings.
func (h *AuthHandler) UpdateUser(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 1. Validate Session
	userID, err := GetUserID(req, h.jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized, Body: "Unauthorized"}, nil
	}

	// 2. Parse Body
	var body struct {
		BaseFolderID string `json:"base_folder_id"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadRequest, Body: "Invalid request body"}, nil
	}

	// 3. Update BaseFolderID
	if body.BaseFolderID != "" {
		if err := h.authService.UpdateBaseFolderID(ctx, userID, body.BaseFolderID); err != nil {
			fmt.Printf("UpdateBaseFolderID error: %v\n", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to update user settings"}, nil
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"success":true}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// DemoLogin issues a temporary JWT without Google OAuth.
func (h *AuthHandler) DemoLogin(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Generate a random demo user ID
	userID := fmt.Sprintf("demo-user-%s", uuid.New().String())
	email := "demo@gophdrive.local"

	// Get Storage Adapter for this user to create root folder
	storage, err := h.storageProvider.GetAdapter(ctx, userID)
	if err != nil {
		fmt.Printf("DemoLogin GetAdapter error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to get storage adapter"}, nil
	}

	// Create Root Folder
	rootFolderID, err := storage.EnsureRootFolder(ctx, "Demo Notes")
	if err != nil {
		fmt.Printf("DemoLogin EnsureRootFolder error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to create root folder"}, nil
	}

	// Save dummy user token to DynamoDB so that GetUser works (and BaseFolderID is set)
	dummyToken := &xoauth2.Token{
		AccessToken:  "dummy-access-token",
		RefreshToken: "dummy-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
		TokenType:    "Bearer",
	}

	// We must save the token first to establish the user, but we also need to update BaseFolderID.
	// SaveToken saves the token. UpdateBaseFolderID updates the setting.
	// Or we can modify authService to allow saving with BaseFolderID, or just call UpdateBaseFolderID after.
	if err := h.authService.SaveToken(ctx, userID, dummyToken); err != nil {
		fmt.Printf("DemoLogin SaveToken error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to save demo user token"}, nil
	}

	if err := h.authService.UpdateBaseFolderID(ctx, userID, rootFolderID); err != nil {
		fmt.Printf("DemoLogin UpdateBaseFolderID error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to set base folder ID"}, nil
	}

	// Create Welcome Notes
	welcomeNotes := []struct {
		Name    string
		Content string
	}{
		{
			Name: "Welcome!.md",
			Content: `# Welcome to GophDrive!

This is a demo of GophDrive, a secure, serverless Markdown note-taking application that uses Google Drive for storage!

## Key Features
- **Google Drive Integration** Your notes are safely stored in your own Google Drive
- **Serverless** Built on AWS for high availability and scalability
- **WebAssembly** Core logic (Markdown rendering, conflict resolution) is written in Go and runs fast in your browser
- **Offline Support** View and edit your notes even without an internet connection (syncs when back online)

## Markdown Syntax
Here are some examples of Markdown elements you can use:

### Text Decoration
- **Bold text** for emphasis
- *Italic text* for subtle emphasis
- ~~Strikethrough~~ to indicate removed content
- ` + "`" + `Inline code` + "`" + ` for technical terms

### Tables
| Feature | Status | Description |
| :--- | :--- | :--- |
| Preview | Active | Fast rendering via WebAssembly |
| Sync | Active | Automatic Google Drive synchronization |
| Security | Active | Serverless encryption for tokens |

### Lists
- Item 1
- Item 2
    - Nested item
- Item 3

### Checklists
- [x] Open the app
- [ ] Write a note
- [ ] Save it

### Code Blocks
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, GophDrive!")
}
` + "```" + `

### Blockquotes
> This is a blockquote. Perfect for highlighting important information.

---
Enjoy exploring GophDrive!`,
		},
		{
			Name: "ようこそ!.md",
			Content: `# GophDrive へようこそ！

これは、Google Drive をストレージとして利用する、セキュアでサーバーレスなマークダウンノートアプリのデモです。

## 主な特徴
- **Google Drive 連携** あなたのノートは、あなたの Google Drive に安全に保管
- **サーバーレス** AWS 上で動作し、高い可用性とスケーラビリティを実現
- **WebAssembly** コアロジック（マークダウン変換や競合解決）は Go で書かれ、ブラウザ上で高速に動作
- **オフライン対応** インターネットがなくてもノートの閲覧・編集が可能（同期はオンライン復帰時）

## マークダウン記法

### テキスト装飾
- **太字** による強調
- *斜体* による控えめな強調
- ~~打ち消し線~~ による削除の表現
- ` + "`" + `インラインコード` + "`" + ` による技術用語の記述

### テーブル
| 機能 | 状態 | 備考 |
| :--- | :--- | :--- |
| プレビュー | 有効 | WebAssemblyによる高速表示 |
| 同期 | 有効 | Google Driveとの自動連携 |
| セキュリティ | 有効 | サーバーレスな暗号化保護 |

### リスト
- 項目 1
- 項目 2
    - ネストされた項目
- 項目 3

### チェックリスト
- [x] アプリを開く
- [ ] ノートを書く
- [ ] 保存する

### コードブロック
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, GophDrive!")
}
` + "```" + `

### 引用
> これは引用文です。重要な情報を強調するのに適しています。

---
さあ、自由にノートを作成して GophDrive を体験してみてください！`,
		},
	}

	for _, note := range welcomeNotes {
		if _, err := storage.CreateFile(ctx, note.Name, []byte(note.Content), rootFolderID); err != nil {
			fmt.Printf("DemoLogin CreateFile (%s) error: %v\n", note.Name, err)
			// Continue even if file creation fails for one
		}
	}

	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"name":  "Demo User",
		"exp":   time.Now().Add(1 * time.Hour).Unix(), // 1 hour session for demo
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signedToken, err := jwtToken.SignedString([]byte(h.jwtSecret))
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Failed to sign token"}, nil
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	cookie := fmt.Sprintf("session_token=%s; HttpOnly; Path=/; Max-Age=86400; SameSite=Lax; Secure", signedToken)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusFound,
		Headers: map[string]string{
			"Location": fmt.Sprintf("%s/?token=%s", frontendURL, signedToken),
		},
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {cookie},
		},
	}, nil
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// SameSite should match Login/DemoLogin
	sameSite := "Lax"
	if os.Getenv("DEV_MODE") != "true" {
		sameSite = "None"
	}

	cookie := fmt.Sprintf("session_token=; HttpOnly; Path=/; Max-Age=0; SameSite=%s; Secure", sameSite)

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       `{"success":true}`,
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {cookie},
		},
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}
