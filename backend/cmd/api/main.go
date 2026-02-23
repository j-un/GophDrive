package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jun/gophdrive/backend/internal/app"
)

func main() {
	application := app.NewApp(context.Background())
	lambda.Start(application.HandleRequest)
}
