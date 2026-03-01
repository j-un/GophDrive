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
			Name: "Welcome (English).md",
			Content: `# GophDrive Overview

A demo of GophDrive, a secure, serverless Markdown note-taking application utilizing Google Drive for storage.

## Key Features
- **Google Drive Integration** Notes are stored securely in a designated Google Drive folder
- **Serverless Architecture** Built on AWS Lambda and DynamoDB for high availability and scalability
- **WebAssembly Execution** Core logic is written in Go and compiled to WebAssembly for performance in the browser
- **Offline Capabilities** Enables viewing and editing without internet connectivity with synchronization upon reconnection

## System Architecture
| Component | Tech Stack | Primary Role |
| :--- | :--- | :--- |
| Frontend | Next.js, TypeScript | UI and WebAssembly execution |
| Backend | Go, AWS Lambda | Google Drive API integration and session management |
| Core Logic | Go (Wasm) | Markdown processing and conflict resolution |
| Database | DynamoDB | Session locks and metadata management |

## Markdown Demonstration

### Lists
- Item 1
- Item 2
    - Nested item
- Item 3

### Numbered Lists
1. First step
2. Second step
3. Final step

### Checklists
- [x] Launch application
- [ ] Create a note
- [ ] Save changes

### Code Blocks
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("GophDrive initialized")
}
` + "```" + `

### Blockquotes
> Critical information can be visually highlighted using blockquotes.

---
Explore the features of GophDrive through this demonstration.`,
		},
		{
			Name: "ようこそ (Japanese).md",
			Content: `# GophDrive 概要

Google Driveを外部ストレージとして利用する、サーバーレスなマークダウンノートアプリケーションのデモ版である。

## 主要な機能
- **Google Drive 連携** ユーザー自身のGoogle Driveにノートが保存されるため、データの制御権を維持できる
- **サーバーレスアーキテクチャ** AWS LambdaおよびDynamoDBを利用し、高い可用性とスケーラビリティを実現している
- **WebAssemblyによる高速処理** マークダウン変換や同期ロジックをWebAssemblyで実行することで、ブラウザ上での高速な動作を可能にしている
- **オフライン編集** インターネット接続が切断された状態でも編集を継続でき、復帰時に同期が行われる

## システム構成
| コンポーネント | 技術スタック | 主要な役割 |
| :--- | :--- | :--- |
| フロントエンド | Next.js, TypeScript | ユーザーインターフェースおよびWasmの実行 |
| バックエンド | Go, AWS Lambda | Google Drive APIとの連携およびセッション管理 |
| コアロジック | Go (Wasm) | マークダウン処理および競合解決 |
| データベース | DynamoDB | セッションロックおよびメタデータの管理 |

## マークダウン要素の検証

### リスト
- 項目 1
- 項目 2
    - 下位項目
- 項目 3

### 番号付きリスト
1. 第一段階
2. 第二段階
3. 最終段階

### チェックリスト
- [x] アプリケーションの起動
- [ ] 新規ノートの作成
- [ ] データの保存

### コードブロック
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("GophDrive initialized")
}
` + "```" + `

### 引用
> 重要な情報は引用ブロックを用いることで視覚的に強調することが可能である。

---
本デモを通じて、GophDriveの機能を自由に検証されたい。`,
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
