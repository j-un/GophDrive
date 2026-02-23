#!/bin/bash
set -e

echo "Starting AWS Deployment Process..."

# ---- Validation ----
if [ -z "${GOOGLE_CLIENT_ID}" ] || [ -z "${GOOGLE_CLIENT_SECRET}" ]; then
  echo -e "\033[0;31mError: Missing required environment variables.\033[0m"
  echo "You must set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to deploy the application."
  echo ""
  echo "How to obtain these credentials:"
  echo "1. Go to Google Cloud Console (https://console.cloud.google.com)"
  echo "2. Create a project and navigate to APIs & Services > Credentials"
  echo "3. Create an OAuth client ID (Web application)"
  echo "4. After deployment, make sure to add the generated CloudFront domain to the Authorized JavaScript origins and Redirect URIs."
  echo ""
  echo "Example:"
  echo "  export GOOGLE_CLIENT_ID=\"your-client-id\""
  echo "  export GOOGLE_CLIENT_SECRET=\"your-client-secret\""
  echo "  ./scripts/deploy-aws.sh"
  exit 1
fi

if [ -n "${CUSTOM_DOMAIN_NAME}" ]; then
  if [ -z "${CERTIFICATE_ARN}" ]; then
    echo -e "\033[0;31mError: CERTIFICATE_ARN is required when CUSTOM_DOMAIN_NAME is set.\033[0m"
    exit 1
  fi
  echo "Custom domain detected: ${CUSTOM_DOMAIN_NAME}"
  export FRONTEND_URL="https://${CUSTOM_DOMAIN_NAME}"
fi

# Note: NEXT_PUBLIC_API_URL should be empty for CloudFront proxying (relative paths)
export NEXT_PUBLIC_API_URL="/api"

# ---- SSM Parameter Store: Manage Secrets ----
echo "Managing secrets in SSM Parameter Store..."

# JWT_SECRET and API_GATEWAY_SECRET: auto-generate if not already in SSM
for PARAM in "/gophdrive/jwt-secret" "/gophdrive/api-gateway-secret"; do
  if ! aws ssm get-parameter --name "$PARAM" > /dev/null 2>&1; then
    echo "  Creating $PARAM (auto-generated)..."
    aws ssm put-parameter --name "$PARAM" \
      --value "$(openssl rand -base64 32)" --type SecureString
  else
    echo "  ✅ $PARAM already exists."
  fi
done

# GOOGLE_CLIENT_SECRET: write only if provided via env var
if [ -n "${GOOGLE_CLIENT_SECRET}" ]; then
  echo "  Writing /gophdrive/google-client-secret from env var..."
  aws ssm put-parameter --name "/gophdrive/google-client-secret" \
    --value "${GOOGLE_CLIENT_SECRET}" --type SecureString --overwrite
fi

# Fetch API_GATEWAY_SECRET from SSM and export for CDK (CloudFront X-Origin-Verify header)
export API_GATEWAY_SECRET=$(aws ssm get-parameter --name "/gophdrive/api-gateway-secret" \
  --with-decryption --query "Parameter.Value" --output text)
echo "  ✅ API_GATEWAY_SECRET fetched from SSM for CloudFront."

echo "SSM secrets ready."

# 1. Build Backend (Lambda Bootstrap)
echo "Building Backend..."
cd backend
GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap ./cmd/api
cd ..

# 2. Deploy Infrastructure
echo "Deploying Infrastructure..."
cd infra
npm install
# Ensure context is set if needed or just deploy
npx cdk deploy --all --require-approval never
cd ..

# 3. Automate Frontend URL and Bucket Extraction
echo "Extracting Deployment Outputs..."
if [ -z "${CUSTOM_DOMAIN_NAME}" ]; then
  export FRONTEND_URL=$(aws cloudformation describe-stacks --stack-name GophDriveFrontendStack --query "Stacks[0].Outputs[?OutputKey=='FrontendUrl'].OutputValue" --output text || echo "")
fi
export BUCKET_NAME=$(aws cloudformation describe-stacks --stack-name GophDriveFrontendStack --query "Stacks[0].Outputs[?OutputKey=='FrontendBucketName'].OutputValue" --output text || echo "")

if [ -z "$FRONTEND_URL" ] || [ -z "$BUCKET_NAME" ]; then
    echo -e "\033[0;31mError: Failed to extract Frontend URL or Bucket Name from CDK outputs. Ensure GophDriveFrontendStack deployed successfully.\033[0m"
    exit 1
fi
echo "  Frontend URL: $FRONTEND_URL"
echo "  S3 Bucket: $BUCKET_NAME"

# 4. Build Frontend
echo "Building Frontend with real FRONTEND_URL..."
cd frontend
rm -rf out .next
npm install
npm run build
cd ..

# 5. Upload Frontend Assets
echo "Uploading Static Assets to S3..."
aws s3 sync frontend/out "s3://$BUCKET_NAME" --delete

echo "Invalidating CloudFront Cache..."
# Get Distribution ID (Assuming only one distribution or filter by tag if possible, but for MVP grabbing the first one)
DIST_ID=$(aws cloudfront list-distributions --query "DistributionList.Items[0].Id" --output text)
if [ "$DIST_ID" != "None" ] && [ -n "$DIST_ID" ]; then
    aws cloudfront create-invalidation --distribution-id $DIST_ID --paths "/*"
    echo "Invalidation created for $DIST_ID"
else
    echo "Could not find CloudFront Distribution ID to invalidate."
fi

echo "Deployment Complete!"
