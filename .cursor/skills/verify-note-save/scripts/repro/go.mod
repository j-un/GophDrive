module github.com/jun/gophdrive/backend/internal/adapter/dynamo/lostupdate

go 1.27

require (
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.74.0
	github.com/jun/gophdrive/backend v0.0.0
)

require (
	github.com/alecthomas/chroma/v3 v3.0.0-alpha.5 // indirect
	github.com/aws/aws-sdk-go-v2 v1.48.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.21.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.47.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.12 // indirect
	github.com/aws/smithy-go v1.28.9 // indirect
	github.com/dlclark/regexp2/v2 v2.13.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jun/gophdrive/core v0.0.0-00010101000000-000000000000 // indirect
	github.com/yuin/goldmark-highlighting/v3 v3.1.1 // indirect
	github.com/yuin/goldmark/v2 v2.1.16 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/jun/gophdrive/backend => ../../../../../backend

replace github.com/jun/gophdrive/core => ../../../../../core
