package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/jun/gophdrive/backend/internal/adapter"
	"github.com/jun/gophdrive/backend/internal/adapter/googledrive"
	"github.com/jun/gophdrive/backend/internal/adapter/memory"
	"github.com/jun/gophdrive/backend/internal/auth"
	"github.com/jun/gophdrive/backend/internal/crypto"
	"github.com/jun/gophdrive/backend/internal/handler"
	"github.com/jun/gophdrive/backend/internal/secret"
	"github.com/jun/gophdrive/backend/internal/session"
)

// HybridProvider delegates to either Google Drive or Memory provider based on user ID.
type HybridProvider struct {
	googleProvider adapter.StorageProvider
	memoryProvider adapter.StorageProvider
}

func (h *HybridProvider) GetAdapter(ctx context.Context, userID string) (adapter.StorageAdapter, error) {
	if strings.HasPrefix(userID, "demo-user-") {
		return h.memoryProvider.GetAdapter(ctx, userID)
	}
	return h.googleProvider.GetAdapter(ctx, userID)
}

// App holds the dependencies for the Lambda function.
type App struct {
	authHandler      *handler.AuthHandler
	noteHandler      *handler.NoteHandler
	sessionHandler   *handler.SessionHandler
	syncHandler      *handler.SyncHandler
	searchHandler    *handler.SearchHandler
	apiGatewaySecret string
}

// NewApp initializes the application dependencies.
func NewApp(ctx context.Context) *App {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("unable to load SDK config, %v", err))
	}

	// DynamoDB Client
	dynamoClient := dynamodb.NewFromConfig(cfg)
	if os.Getenv("DEV_MODE") == "true" {
		fmt.Println("Using In-Memory/DynamoDB Hybrid Storage (DEV_MODE=true)")
	}

	// KMS Client
	var kmsService crypto.Encryptor
	if os.Getenv("DEV_MODE") == "true" {
		kmsService = crypto.NewMockEncryptor()
		fmt.Println("Using MockEncryptor (DEV_MODE=true)")
	} else {
		kmsClient := kms.NewFromConfig(cfg)
		kmsKeyID := os.Getenv("KMS_KEY_ID")
		if kmsKeyID == "" {
			kmsKeyID = "alias/gophdrive-token-key"
		}
		kmsService = crypto.NewKMSService(kmsClient, kmsKeyID)
	}

	// Auth Service (UserTokens Table)
	userTokensTable := os.Getenv("USER_TOKENS_TABLE")
	if userTokensTable == "" {
		userTokensTable = "UserTokens"
	}

	// ---------- Secret Resolver ----------
	var resolver secret.Resolver
	if os.Getenv("DEV_MODE") == "true" {
		resolver = secret.NewEnvResolver()
		fmt.Println("Using EnvResolver (DEV_MODE=true)")
	} else {
		resolver = secret.NewSSMResolver(ssm.NewFromConfig(cfg))
		fmt.Println("Using SSMResolver (SSM Parameter Store)")
	}

	// Resolve secrets from SSM Parameter Store (or env vars in DEV_MODE)
	googleClientSecretParam := os.Getenv("GOOGLE_CLIENT_SECRET_PARAM")
	if googleClientSecretParam == "" {
		googleClientSecretParam = "/gophdrive/google-client-secret"
	}
	googleClientSecret, err := resolver.GetSecret(ctx, googleClientSecretParam)
	if err != nil {
		log.Printf("WARNING: failed to resolve GOOGLE_CLIENT_SECRET: %v", err)
	}

	jwtSecretParam := os.Getenv("JWT_SECRET_PARAM")
	if jwtSecretParam == "" {
		jwtSecretParam = "/gophdrive/jwt-secret"
	}
	jwtSecret, err := resolver.GetSecret(ctx, jwtSecretParam)
	if err != nil {
		log.Printf("WARNING: failed to resolve JWT_SECRET: %v", err)
		jwtSecret = "default-dev-secret"
	}

	apiGatewaySecretParam := os.Getenv("API_GATEWAY_SECRET_PARAM")
	if apiGatewaySecretParam == "" {
		apiGatewaySecretParam = "/gophdrive/api-gateway-secret"
	}
	apiGatewaySecret, err := resolver.GetSecret(ctx, apiGatewaySecretParam)
	if err != nil {
		log.Printf("WARNING: failed to resolve API_GATEWAY_SECRET: %v", err)
	}

	// OAuth2 Config
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if redirectURL == "" {
		if os.Getenv("DEV_MODE") == "true" {
			redirectURL = "http://localhost:8080/auth/callback"
		} else {
			frontendURL := os.Getenv("FRONTEND_URL")
			if frontendURL == "" {
				frontendURL = "http://localhost:3000"
			}
			redirectURL = frontendURL + "/api/auth/callback"
		}
	}

	oauthConfig := &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: googleClientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/drive",
			"https://www.googleapis.com/auth/userinfo.email",
		},
		Endpoint: google.Endpoint,
	}

	authService := auth.NewAuthService(oauthConfig, dynamoClient, userTokensTable, kmsService)
	// Storage Provider
	var storageProvider adapter.StorageProvider
	if os.Getenv("DEV_MODE") == "true" {
		// Use DynamoDB-backed "Memory" provider for persistence in LocalStack
		storageProvider = memory.NewProvider(dynamoClient, authService)
		fmt.Println("Using MemoryProvider (DEV_MODE=true) with DynamoDB persistence")
	} else {
		// Production: Hybrid Provider (Google Drive + Demo Memory)
		storageProvider = &HybridProvider{
			googleProvider: googledrive.NewProvider(authService),
			memoryProvider: memory.NewProvider(dynamoClient, authService),
		}
	}

	// Auth Handler (needs Auth Service and Storage Provider)
	authHandler := handler.NewAuthHandler(authService, storageProvider, jwtSecret)

	// Note Handler
	noteHandler := handler.NewNoteHandler(storageProvider, jwtSecret)

	// Search Handler
	searchHandler := handler.NewSearchHandler(storageProvider, jwtSecret)

	// Session Manager (EditingSessions Table)
	sessionsTable := os.Getenv("EDITING_SESSIONS_TABLE")
	if sessionsTable == "" {
		sessionsTable = "EditingSessions"
	}
	lockManager := session.NewLockManager(dynamoClient, sessionsTable)
	sessionHandler := handler.NewSessionHandler(lockManager, jwtSecret)

	// Sync Handler
	syncHandler := handler.NewSyncHandler(jwtSecret)

	return &App{
		authHandler:      authHandler,
		noteHandler:      noteHandler,
		sessionHandler:   sessionHandler,
		syncHandler:      syncHandler,
		searchHandler:    searchHandler,
		apiGatewaySecret: apiGatewaySecret,
	}
}

// HandleRequest routes API Gateway requests to the appropriate handler.
func (app *App) HandleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := req.Path
	method := req.HTTPMethod

	fmt.Printf("Request: %s %s\n", method, path)

	// CORS Preflight
	if method == "OPTIONS" {
		return corsResponse(events.APIGatewayProxyResponse{StatusCode: 204}), nil
	}

	// Security: Verify Request Origin (CloudFront only)
	// Skip check for OPTIONS (preflight) and if DEV_MODE is true
	if os.Getenv("DEV_MODE") != "true" {
		if req.Headers["X-Origin-Verify"] != app.apiGatewaySecret && req.Headers["x-origin-verify"] != app.apiGatewaySecret {
			fmt.Printf("Security Block: Missing or invalid X-Origin-Verify header\n")
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusForbidden,
				Body:       "Forbidden: Access denied",
			}, nil
		}
	}

	// Router logic
	// Strip /api prefix if present (for CloudFront proxying)
	if strings.HasPrefix(path, "/api") {
		path = strings.TrimPrefix(path, "/api")
	}

	// Helper to ensure PathParameters is initialized
	if req.PathParameters == nil {
		req.PathParameters = make(map[string]string)
	}

	// /auth
	if strings.HasPrefix(path, "/auth") {
		if path == "/auth/login" && method == "GET" {
			return corsResponse(must(app.authHandler.Login(ctx, req))), nil
		}
		if path == "/auth/callback" && method == "GET" {
			return corsResponse(must(app.authHandler.Callback(ctx, req))), nil
		}
		if path == "/auth/demo-login" && method == "GET" {
			return corsResponse(must(app.authHandler.DemoLogin(ctx, req))), nil
		}
		if path == "/auth/logout" && method == "POST" {
			return corsResponse(must(app.authHandler.Logout(ctx, req))), nil
		}
		if path == "/auth/drive/folders" && method == "GET" {
			return corsResponse(must(app.authHandler.ListDriveFolders(ctx, req))), nil
		}
		if path == "/auth/user" && method == "GET" {
			return corsResponse(must(app.authHandler.GetUser(ctx, req))), nil
		}
		if path == "/auth/user" && method == "PATCH" {
			return corsResponse(must(app.authHandler.UpdateUser(ctx, req))), nil
		}
	}

	// /notes
	if strings.HasPrefix(path, "/notes") {
		if path == "/notes" && method == "GET" {
			return corsResponse(must(app.noteHandler.ListNotes(ctx, req))), nil
		}
		if path == "/notes" && method == "POST" {
			return corsResponse(must(app.noteHandler.CreateNote(ctx, req))), nil
		}
		// /notes/{id}
		if len(path) > len("/notes/") {
			// Robust ID extraction
			pathParts := strings.Split(strings.Trim(path, "/"), "/")
			id := pathParts[len(pathParts)-1]
			req.PathParameters["id"] = id

			if method == "GET" {
				return corsResponse(must(app.noteHandler.GetNote(ctx, req))), nil
			}
			if method == "PUT" {
				return corsResponse(must(app.noteHandler.UpdateNote(ctx, req))), nil
			}
			if method == "POST" && strings.HasSuffix(path, "/delete") {
				// Handle POST /notes/{id}/delete
				if len(pathParts) >= 2 && pathParts[len(pathParts)-1] == "delete" {
					req.PathParameters["id"] = pathParts[len(pathParts)-2]
				}
				return corsResponse(must(app.noteHandler.DeleteNote(ctx, req))), nil
			}
			if method == "POST" && strings.HasSuffix(path, "/copy") {
				// Handle POST /notes/{id}/copy
				if len(pathParts) >= 2 && pathParts[len(pathParts)-1] == "copy" {
					req.PathParameters["id"] = pathParts[len(pathParts)-2]
				}
				return corsResponse(must(app.noteHandler.DuplicateNote(ctx, req))), nil
			}

			if method == "PATCH" {
				return corsResponse(must(app.noteHandler.PatchNote(ctx, req))), nil
			}

			if method == "DELETE" {
				return corsResponse(must(app.noteHandler.DeleteNote(ctx, req))), nil
			}
		}
	}

	// /starred
	if path == "/starred" && method == "GET" {
		return corsResponse(must(app.noteHandler.ListStarredNotes(ctx, req))), nil
	}

	// /folders
	if path == "/folders" && method == "POST" {
		return corsResponse(must(app.noteHandler.CreateFolder(ctx, req))), nil
	}

	// /sessions
	if strings.HasPrefix(path, "/sessions/") {
		parts := strings.Split(strings.TrimPrefix(path, "/sessions/"), "/")
		if len(parts) >= 2 {
			req.PathParameters["fileId"] = parts[0]
			action := parts[1]

			if action == "lock" {
				if method == "POST" {
					return corsResponse(must(app.sessionHandler.AcquireLock(ctx, req))), nil
				}
				if method == "DELETE" {
					return corsResponse(must(app.sessionHandler.ReleaseLock(ctx, req))), nil
				}
			}
			if action == "heartbeat" && method == "POST" {
				return corsResponse(must(app.sessionHandler.Heartbeat(ctx, req))), nil
			}
		}
	}

	// /sync
	if path == "/sync/check" && method == "POST" {
		return corsResponse(must(app.syncHandler.CheckConflict(ctx, req))), nil
	}

	// /search
	if path == "/search" && method == "GET" {
		return corsResponse(must(app.searchHandler.Search(ctx, req))), nil
	}

	return corsResponse(events.APIGatewayProxyResponse{
		StatusCode: http.StatusNotFound,
		Body:       fmt.Sprintf("Not Found: %s %s", method, path),
	}), nil
}

// corsResponse adds CORS headers to an API Gateway response.
func corsResponse(resp events.APIGatewayProxyResponse) events.APIGatewayProxyResponse {
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	resp.Headers["Access-Control-Allow-Origin"] = os.Getenv("FRONTEND_URL")
	if resp.Headers["Access-Control-Allow-Origin"] == "" {
		resp.Headers["Access-Control-Allow-Origin"] = "http://localhost:3000"
	}
	resp.Headers["Access-Control-Allow-Credentials"] = "true"
	resp.Headers["Access-Control-Allow-Methods"] = "GET,POST,PUT,DELETE,OPTIONS,PATCH"
	resp.Headers["Access-Control-Allow-Headers"] = "Content-Type,Authorization,If-Match"
	return resp
}

// must unwraps a handler response, ignoring the error.
func must(resp events.APIGatewayProxyResponse, err error) events.APIGatewayProxyResponse {
	if err != nil {
		fmt.Printf("Handler error: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError, Body: "Internal Server Error"}
	}
	return resp
}
