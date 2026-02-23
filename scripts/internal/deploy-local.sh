#!/bin/bash
set -euo pipefail
export AWS_PAGER=""

echo "🚀 Deploying infrastructure to LocalStack via AWS CLI..."

ENDPOINT_URL="${ENDPOINT_URL:-${AWS_ENDPOINT_URL:-http://localstack:4566}}"
REGION="ap-northeast-1"
AWS_CMD="aws --endpoint-url=${ENDPOINT_URL} --region=${REGION}"

# Function to check if table exists
table_exists() {
    $AWS_CMD dynamodb describe-table --table-name "$1" >/dev/null 2>&1
}

# 1. Create UserTokens Table
if table_exists "UserTokens"; then
    echo "✅ Table UserTokens already exists."
else
    echo "📦 Creating UserTokens table..."
    $AWS_CMD dynamodb create-table \
        --table-name UserTokens \
        --attribute-definitions AttributeName=user_id,AttributeType=S \
        --key-schema AttributeName=user_id,KeyType=HASH \
        --billing-mode PAY_PER_REQUEST
    
    # Enable PITR (Point-In-Time Recovery) - simulated
    $AWS_CMD dynamodb update-continuous-backups \
        --table-name UserTokens \
        --point-in-time-recovery-specification PointInTimeRecoveryEnabled=true
fi

# 2. Create EditingSessions Table
if table_exists "EditingSessions"; then
    echo "✅ Table EditingSessions already exists."
else
    echo "📦 Creating EditingSessions table..."
    $AWS_CMD dynamodb create-table \
        --table-name EditingSessions \
        --attribute-definitions AttributeName=file_id,AttributeType=S \
        --key-schema AttributeName=file_id,KeyType=HASH \
        --billing-mode PAY_PER_REQUEST

    $AWS_CMD dynamodb update-time-to-live \
        --table-name EditingSessions \
        --time-to-live-specification Enabled=true,AttributeName=expires_at
fi

# 2.5 Create FileStore Table (for Dev Mode Persistence)
if table_exists "FileStore"; then
    echo "✅ Table FileStore already exists."
else
    echo "📦 Creating FileStore table..."
    $AWS_CMD dynamodb create-table \
        --table-name FileStore \
        --attribute-definitions AttributeName=pk,AttributeType=S \
        --key-schema AttributeName=pk,KeyType=HASH \
        --billing-mode PAY_PER_REQUEST
fi

# 3. Create KMS Key
echo "🔑 Checking/Creating KMS Key..."
# Check for existing alias
EXISTING_KEY_ID=$($AWS_CMD kms list-aliases --query "Aliases[?AliasName=='alias/antigravity-token-key'].TargetKeyId" --output text)

if [ -z "$EXISTING_KEY_ID" ]; then
    echo "   Creating new KMS key..."
    KEY_ID=$($AWS_CMD kms create-key --description "Key for encrypting OAuth2 refresh tokens" --query KeyMetadata.KeyId --output text)
    $AWS_CMD kms create-alias --alias-name "alias/antigravity-token-key" --target-key-id "$KEY_ID"
    $AWS_CMD kms enable-key-rotation --key-id "$KEY_ID"
    echo "   ✅ Created key with alias alias/antigravity-token-key"
else
    echo "   ✅ Key alias alias/antigravity-token-key already exists (KeyID: $EXISTING_KEY_ID)"
fi

# 3.5 Create SSM Parameters for secrets (LocalStack)
echo "🔐 Creating SSM Parameters..."
ssm_param_exists() {
    $AWS_CMD ssm get-parameter --name "$1" > /dev/null 2>&1
}

for PARAM in "/gophdrive/jwt-secret" "/gophdrive/api-gateway-secret"; do
    if ssm_param_exists "$PARAM"; then
        echo "   ✅ SSM parameter $PARAM already exists."
    else
        echo "   Creating $PARAM..."
        $AWS_CMD ssm put-parameter --name "$PARAM" \
            --value "local-dev-$(echo $PARAM | tr '/' '-')" --type SecureString
    fi
done

if ssm_param_exists "/gophdrive/google-client-secret"; then
    echo "   ✅ SSM parameter /gophdrive/google-client-secret already exists."
else
    echo "   Creating /gophdrive/google-client-secret..."
    $AWS_CMD ssm put-parameter --name "/gophdrive/google-client-secret" \
        --value "dummy" --type SecureString
fi

# 4. Deploy Backend Lambda
echo "Functions: Deploying Lambda..."

# Build Go binary
echo "   Building backend..."
cd backend
# Build for Linux ARM64 (LocalStack runs in Docker on ARM64 Mac)
# Use provided.al2023 compatible build
export GOCACHE=/go/.cache
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap ./cmd/api
zip -q function.zip bootstrap
cd ..

# 3.5 Create IAM Role/Policy if needed
echo "👤 Checking IAM Role..."
ROLE_NAME="lambda-role"
if ! $AWS_CMD iam get-role --role-name $ROLE_NAME >/dev/null 2>&1; then
    echo "   Creating IAM role..."
    $AWS_CMD iam create-role --role-name $ROLE_NAME --assume-role-policy-document '{"Version": "2012-10-17","Statement": [{"Effect": "Allow","Principal": {"Service": "lambda.amazonaws.com"},"Action": "sts:AssumeRole"}]}' >/dev/null
    
    echo "   Attaching policies..."
    $AWS_CMD iam attach-role-policy --role-name $ROLE_NAME --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole >/dev/null
    $AWS_CMD iam attach-role-policy --role-name $ROLE_NAME --policy-arn arn:aws:iam::aws:policy/AmazonS3FullAccess >/dev/null
    $AWS_CMD iam attach-role-policy --role-name $ROLE_NAME --policy-arn arn:aws:iam::aws:policy/AmazonDynamoDBFullAccess >/dev/null
    $AWS_CMD iam attach-role-policy --role-name $ROLE_NAME --policy-arn arn:aws:iam::aws:policy/AWSKeyManagementServicePowerUser >/dev/null
fi

ROLE_ARN="arn:aws:iam::000000000000:role/${ROLE_NAME}"

# Check if function exists
if $AWS_CMD lambda get-function --function-name BackendFunction >/dev/null 2>&1; then
    echo "   Updating function code..."
    $AWS_CMD lambda update-function-code --function-name BackendFunction --zip-file fileb://backend/function.zip >/dev/null
    # Update config just in case
    $AWS_CMD lambda update-function-configuration \
        --function-name BackendFunction \
        --environment "Variables={USER_TOKENS_TABLE=UserTokens,EDITING_SESSIONS_TABLE=EditingSessions,KMS_KEY_ID=alias/antigravity-token-key,JWT_SECRET=dev-secret,GOOGLE_CLIENT_SECRET=dummy,DEV_MODE=true,FRONTEND_URL=http://localhost:3000,GOOGLE_CLIENT_ID=dummy,AWS_ENDPOINT_URL=http://localstack:4566}" >/dev/null
else
    echo "   Creating function..."
    $AWS_CMD lambda create-function \
        --function-name BackendFunction \
        --runtime provided.al2023 \
        --architectures arm64 \
        --handler bootstrap \
        --role $ROLE_ARN \
        --zip-file fileb://backend/function.zip \
        --environment "Variables={USER_TOKENS_TABLE=UserTokens,EDITING_SESSIONS_TABLE=EditingSessions,KMS_KEY_ID=alias/antigravity-token-key,JWT_SECRET=dev-secret,GOOGLE_CLIENT_SECRET=dummy,DEV_MODE=true,FRONTEND_URL=http://localhost:3000,GOOGLE_CLIENT_ID=dummy,AWS_ENDPOINT_URL=http://localstack:4566}" >/dev/null
fi
echo "   ✅ BackendFunction deployed."

# 5. Create API Gateway (REST API)
echo "🚪 Checking/Deploying API Gateway..."
API_NAME="GophDriveAPI"
API_ID=$($AWS_CMD apigateway get-rest-apis --query "items[?name=='${API_NAME}'].id" --output text)

if [ -z "$API_ID" ] || [ "$API_ID" == "None" ]; then
    API_ID=$($AWS_CMD apigateway create-rest-api --name "${API_NAME}" --query id --output text)
    echo "   Created REST API: $API_ID"
else
    echo "   Existing REST API: $API_ID"
fi

# Get Root Resource ID
ROOT_ID=$($AWS_CMD apigateway get-resources --rest-api-id $API_ID --query "items[?path=='/'].id" --output text)

# For Proxy Integration on Root
# We need to act carefully to avoid errors if methods exist.
# Simplest approach for LocalStack: Create a catch-all proxy resource if not exists.
# Check for ANY method on root
if ! $AWS_CMD apigateway get-method --rest-api-id $API_ID --resource-id $ROOT_ID --http-method ANY >/dev/null 2>&1; then
    $AWS_CMD apigateway put-method \
        --rest-api-id $API_ID \
        --resource-id $ROOT_ID \
        --http-method ANY \
        --authorization-type NONE >/dev/null

    $AWS_CMD apigateway put-integration \
        --rest-api-id $API_ID \
        --resource-id $ROOT_ID \
        --http-method ANY \
        --type AWS_PROXY \
        --integration-http-method POST \
        --uri arn:aws:apigateway:${REGION}:lambda:path/2015-03-31/functions/arn:aws:lambda:${REGION}:000000000000:function:BackendFunction/invocations >/dev/null
fi

# Check for {proxy+} resource
PROXY_ID=$($AWS_CMD apigateway get-resources --rest-api-id $API_ID --query "items[?pathPart=='{proxy+}'].id" --output text)

if [ -z "$PROXY_ID" ] || [ "$PROXY_ID" == "None" ]; then
    PROXY_ID=$($AWS_CMD apigateway create-resource \
        --rest-api-id $API_ID \
        --parent-id $ROOT_ID \
        --path-part "{proxy+}" \
        --query id --output text)
    echo "   Created Proxy Resource: $PROXY_ID"
fi

# Put Methods on {proxy+}
for METHOD in GET POST PUT DELETE OPTIONS PATCH; do
    if ! $AWS_CMD apigateway get-method --rest-api-id $API_ID --resource-id $PROXY_ID --http-method $METHOD >/dev/null 2>&1; then
        $AWS_CMD apigateway put-method \
            --rest-api-id $API_ID \
            --resource-id $PROXY_ID \
            --http-method $METHOD \
            --authorization-type NONE >/dev/null

        $AWS_CMD apigateway put-integration \
            --rest-api-id $API_ID \
            --resource-id $PROXY_ID \
            --http-method $METHOD \
            --type AWS_PROXY \
            --integration-http-method POST \
            --uri arn:aws:apigateway:${REGION}:lambda:path/2015-03-31/functions/arn:aws:lambda:${REGION}:000000000000:function:BackendFunction/invocations >/dev/null
    fi
done

# Deployment
echo "   Deploying API..."
$AWS_CMD apigateway create-deployment --rest-api-id $API_ID --stage-name dev >/dev/null

echo "🎉 Deployment complete!"
echo "API URL: ${ENDPOINT_URL}/restapis/${API_ID}/dev/_user_request_/"
