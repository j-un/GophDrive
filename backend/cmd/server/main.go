package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jun/gophdrive/backend/internal/app"
)

func main() {
	application := app.NewApp(context.Background())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		headers := make(map[string]string)
		for k, v := range r.Header {
			headers[k] = v[0]
		}

		queryParams := make(map[string]string)
		for k, v := range r.URL.Query() {
			queryParams[k] = v[0]
		}

		req := events.APIGatewayProxyRequest{
			Path:                  r.URL.Path,
			HTTPMethod:            r.Method,
			Headers:               headers,
			QueryStringParameters: queryParams,
			Body:                  string(body),
			IsBase64Encoded:       false,
		}

		resp, err := application.HandleRequest(context.Background(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.StatusCode)
		// Lambda + API Gateway encode binary bodies as base64. Decode here so
		// dev mode delivers proper bytes to the browser (e.g. ZIP downloads).
		if resp.IsBase64Encoded {
			decoded, err := base64.StdEncoding.DecodeString(resp.Body)
			if err != nil {
				log.Printf("base64 decode failed: %v", err)
				return
			}
			w.Write(decoded)
			return
		}
		w.Write([]byte(resp.Body))
	})

	fmt.Println("Starting local server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
