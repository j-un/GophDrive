package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/jun/gophdrive/backend/internal/adapter/apikey"
	"github.com/jun/gophdrive/backend/internal/adapter/dynamo"
	"github.com/jun/gophdrive/backend/internal/auth"
	"github.com/jun/gophdrive/backend/internal/handler"
	"github.com/jun/gophdrive/backend/internal/secret"
	"github.com/jun/gophdrive/backend/internal/session"
)

// App holds the dependencies for the Lambda function.
type App struct {
	authHandler      *handler.AuthHandler
	noteHandler      *handler.NoteHandler
	sessionHandler   *handler.SessionHandler
	syncHandler      *handler.SyncHandler
	searchHandler    *handler.SearchHandler
	exportHandler    *handler.ExportHandler
	tagHandler       *handler.TagHandler
	graphHandler     *handler.GraphHandler
	apiKeyHandler    *handler.APIKeyHandler
	apiKeys          apikey.Store
	apiGatewaySecret string
	frontendURL      string
	jwtSecret        string
}

// getRequestHeader returns the first value for the given header name, checking
// both req.Headers and req.MultiValueHeaders case-insensitively.
func getRequestHeader(req events.APIGatewayProxyRequest, name string) string {
	for k, v := range req.Headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	for k, v := range req.MultiValueHeaders {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// NewApp initializes the application dependencies.
func NewApp(ctx context.Context) *App {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("unable to load SDK config, %v", err))
	}

	devMode := os.Getenv("DEV_MODE") == "true"

	dynamoClient := dynamodb.NewFromConfig(cfg)
	if devMode {
		fmt.Println("Using DynamoDB-backed storage (DEV_MODE=true)")
	}

	// ---------- Secret Resolver ----------
	var resolver secret.Resolver
	if devMode {
		resolver = secret.NewEnvResolver()
		fmt.Println("Using EnvResolver (DEV_MODE=true)")
	} else {
		resolver = secret.NewSSMResolver(ssm.NewFromConfig(cfg))
		fmt.Println("Using SSMResolver (SSM Parameter Store)")
	}

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

	apiKeyStore := apikey.NewDDBStore(dynamoClient, os.Getenv("API_KEY_HASHES_TABLE"))

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")

	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	if redirectURL == "" {
		if devMode {
			redirectURL = "http://localhost:8080/auth/callback"
		} else {
			redirectURL = frontendURL + "/api/auth/callback"
		}
	}

	// OIDC-only scopes — Drive scope was removed when storage moved to DynamoDB.
	oauthConfig := &oauth2.Config{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}

	verifier := auth.NewGoogleVerifier(googleClientID)

	storageProvider := dynamo.NewProvider(dynamoClient)

	var allowedEmails []string
	if raw := os.Getenv("ALLOWED_EMAILS"); raw != "" {
		for _, email := range strings.Split(raw, ",") {
			email = strings.TrimSpace(email)
			if email != "" {
				allowedEmails = append(allowedEmails, email)
			}
		}
	}

	authHandler := handler.NewAuthHandler(handler.AuthHandlerDeps{
		Exchanger:       oauthConfig,
		Verifier:        verifier,
		StorageProvider: storageProvider,
		JWTSecret:       jwtSecret,
		AllowedEmails:   allowedEmails,
		FrontendURL:     frontendURL,
	})

	noteHandler := handler.NewNoteHandler(storageProvider, jwtSecret)
	searchHandler := handler.NewSearchHandler(storageProvider, jwtSecret)
	exportHandler := handler.NewExportHandler(storageProvider, jwtSecret)
	tagHandler := handler.NewTagHandler(storageProvider, jwtSecret)
	graphHandler := handler.NewGraphHandler(storageProvider, jwtSecret)

	sessionsTable := os.Getenv("EDITING_SESSIONS_TABLE")
	if sessionsTable == "" {
		sessionsTable = "EditingSessions"
	}
	lockManager := session.NewLockManager(dynamoClient, sessionsTable)
	sessionHandler := handler.NewSessionHandler(lockManager, jwtSecret)

	syncHandler := handler.NewSyncHandler(jwtSecret)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyStore, jwtSecret)

	return &App{
		authHandler:      authHandler,
		noteHandler:      noteHandler,
		sessionHandler:   sessionHandler,
		syncHandler:      syncHandler,
		searchHandler:    searchHandler,
		exportHandler:    exportHandler,
		tagHandler:       tagHandler,
		graphHandler:     graphHandler,
		apiKeyHandler:    apiKeyHandler,
		apiKeys:          apiKeyStore,
		apiGatewaySecret: apiGatewaySecret,
		frontendURL:      frontendURL,
		jwtSecret:        jwtSecret,
	}
}

// HandleRequest routes API Gateway requests to the appropriate handler.
func (app *App) HandleRequest(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := req.Path
	method := req.HTTPMethod

	fmt.Printf("Request: %s %s\n", method, path)

	if method == "OPTIONS" {
		return corsResponse(events.APIGatewayProxyResponse{StatusCode: 204}), nil
	}

	if os.Getenv("DEV_MODE") != "true" {
		if getRequestHeader(req, "X-Origin-Verify") != app.apiGatewaySecret {
			fmt.Printf("Security Block: Missing or invalid X-Origin-Verify header\n")
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusForbidden,
				Body:       "Forbidden: Access denied",
			}, nil
		}
	}

	if strings.HasPrefix(path, "/api/") {
		path = strings.TrimPrefix(path, "/api")
	}

	// CSRF defense: Cookie-only POST/PUT/PATCH/DELETE requests must carry a matching
	// Origin and X-Requested-With: XMLHttpRequest header. Bearer-authenticated
	// requests are exempted — programmatic clients (CLI, API keys) send an explicit
	// Authorization header and cannot accidentally ride Cookies across origins.
	// NOTE: this check must run before translateAPIKey so it sees the original header.
	if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
		authHeader := getRequestHeader(req, "Authorization")
		if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			origin := getRequestHeader(req, "Origin")
			if origin != app.frontendURL {
				fmt.Printf("Security Block: CSRF — Origin %q != expected %q\n", origin, app.frontendURL)
				return events.APIGatewayProxyResponse{
					StatusCode: http.StatusForbidden,
					Body:       "Forbidden: Bad Origin",
				}, nil
			}
			if getRequestHeader(req, "X-Requested-With") != "XMLHttpRequest" {
				fmt.Printf("Security Block: CSRF — X-Requested-With missing or invalid\n")
				return events.APIGatewayProxyResponse{
					StatusCode: http.StatusForbidden,
					Body:       "Forbidden: Missing X-Requested-With",
				}, nil
			}
		}
	}

	// Translate an API key (Bearer <plaintext>) to a short-lived session JWT so
	// all downstream handlers can verify it via the normal GetSessionClaims path
	// without modification. Revocation is immediate — DynamoDB lookup on every request.
	if newAuth, ok := translateAPIKey(ctx, req.Headers, app.apiKeys, app.jwtSecret); ok {
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		req.Headers["Authorization"] = newAuth
	}

	if req.PathParameters == nil {
		req.PathParameters = make(map[string]string)
	}

	// /api-keys — API key lifecycle for programmatic access
	if path == "/api-keys" {
		if method == "POST" {
			return corsResponse(must(app.apiKeyHandler.Issue(ctx, req))), nil
		}
		if method == "GET" {
			return corsResponse(must(app.apiKeyHandler.Get(ctx, req))), nil
		}
		if method == "DELETE" {
			return corsResponse(must(app.apiKeyHandler.Delete(ctx, req))), nil
		}
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
		if path == "/auth/refresh" && method == "POST" {
			return corsResponse(must(app.authHandler.Refresh(ctx, req))), nil
		}
		if path == "/auth/user" && method == "GET" {
			return corsResponse(must(app.authHandler.GetUser(ctx, req))), nil
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
		if len(path) > len("/notes/") {
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
				if len(pathParts) >= 2 && pathParts[len(pathParts)-1] == "delete" {
					req.PathParameters["id"] = pathParts[len(pathParts)-2]
				}
				return corsResponse(must(app.noteHandler.DeleteNote(ctx, req))), nil
			}
			if method == "POST" && strings.HasSuffix(path, "/copy") {
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

	if path == "/starred" && method == "GET" {
		return corsResponse(must(app.noteHandler.ListStarredNotes(ctx, req))), nil
	}

	if path == "/recent" && method == "GET" {
		return corsResponse(must(app.noteHandler.ListRecentNotes(ctx, req))), nil
	}

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

	if path == "/sync/check" && method == "POST" {
		return corsResponse(must(app.syncHandler.CheckConflict(ctx, req))), nil
	}

	if path == "/search" && method == "GET" {
		return corsResponse(must(app.searchHandler.Search(ctx, req))), nil
	}

	if path == "/tags" && method == "GET" {
		return corsResponse(must(app.tagHandler.ListTags(ctx, req))), nil
	}

	if path == "/export" && method == "GET" {
		return corsResponse(must(app.exportHandler.Export(ctx, req))), nil
	}

	if path == "/graph" && method == "GET" {
		return corsResponse(must(app.graphHandler.Graph(ctx, req))), nil
	}

	return corsResponse(events.APIGatewayProxyResponse{
		StatusCode: http.StatusNotFound,
		Body:       fmt.Sprintf("Not Found: %s %s", method, path),
	}), nil
}

// translateAPIKey checks whether the Authorization header carries a valid API key.
// On match it mints a 5-minute session JWT for the key owner and returns the
// replacement "Bearer <jwt>" header value. Returns ok=false when no key is
// provided, the key is unknown, or signing fails. DynamoDB errors are logged
// and treated as a miss so human sessions are never affected.
// The Authorization header name is checked case-insensitively; the Bearer
// scheme is also accepted in any case (bearer, BEARER, etc.).
func translateAPIKey(ctx context.Context, headers map[string]string, store apikey.Store, jwtSecret string) (string, bool) {
	if store == nil {
		return "", false
	}
	authHeader := headers["Authorization"]
	if authHeader == "" {
		authHeader = headers["authorization"]
	}
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return "", false
	}
	candidate := authHeader[7:] // "bearer " is always 7 bytes after ToLower+HasPrefix normalisation
	hash := apikey.HashKey(candidate)

	userID, baseFolderID, ok, err := store.Lookup(ctx, hash)
	if err != nil {
		log.Printf("WARNING: translateAPIKey: store lookup: %v", err)
		return "", false
	}
	if !ok {
		return "", false
	}

	claims := handler.SessionClaims{UserID: userID, BaseFolderID: baseFolderID}
	tok, err := handler.SignSession(claims, 5*time.Minute, jwtSecret)
	if err != nil {
		log.Printf("WARNING: translateAPIKey: SignSession: %v", err)
		return "", false
	}
	return "Bearer " + tok, true
}

// corsResponse adds CORS headers to an API Gateway response.
func corsResponse(resp events.APIGatewayProxyResponse) events.APIGatewayProxyResponse {
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	resp.Headers["Access-Control-Allow-Origin"] = os.Getenv("FRONTEND_URL")
	if resp.Headers["Access-Control-Allow-Origin"] == "" {
		resp.Headers["Access-Control-Allow-Origin"] = "http://localhost:5173"
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
