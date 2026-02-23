# GophDrive

GophDrive is a highly secure, serverless Markdown note-taking application designed for AWS. It leverages your own Google Drive for storage, ensuring you maintain complete control and privacy over your data.
Built with extensibility in mind, GophDrive uses a clean adapter pattern that allows for future support of alternative storage providers.

## Key Features

- **Google Drive Integration**: Your notes are safely stored as Markdown files directly in a designated folder in your Google Drive.
- **Serverless Architecture**: Built on AWS Lambda, API Gateway, DynamoDB, S3, and CloudFront for high availability, automatic scaling, and low cost.
- **Client-Side Processing (WebAssembly)**: Core logic, including Markdown processing and conflict resolution, is written in Go and compiled to WebAssembly (Wasm) for fast, secure execution directly in your browser.
- **Real-Time Conflict Management**: Session-based locking ensures that concurrent edits don't result in data loss.
- **Demo Mode**: Try out the application temporarily without connecting your Google account using the built-in Ephemeral Storage Demo Mode.
- **Custom Domains**: Easily map your own domain name (with TLS 1.3 enforcement) via the automated AWS CDK deployment scripts.

## Tech Stack

- **Frontend**: Next.js (App Router), React, TypeScript, CSS Modules
- **Backend (API)**: Go (standard library/AWS Lambda Go), compiled for `provided.al2023` ARM64 Lambda
- **Shared Core**: Go (compiled to WebAssembly)
- **Infrastructure**: AWS CDK (TypeScript), LocalStack for local development
- **Database (Meta/Sessions)**: Amazon DynamoDB
- **Auth**: Google OAuth 2.0 + Custom JWTs

## Project Structure

```text
GophDrive/
├── backend/            # Go Backend API (AWS Lambda handlers & business logic)
├── core/               # Shared Go logic (compiled to Wasm for the frontend)
├── frontend/           # Next.js SPA Frontend
├── infra/              # Infrastructure as Code (AWS CDK definitions)
├── scripts/            # Automation scripts for local dev and AWS deployment
└── docker-compose.yml  # Local development environment using LocalStack
```

## Getting Started

### Prerequisites

- [Docker](https://www.docker.com/) and Docker Compose
- [AWS CLI](https://aws.amazon.com/cli/) (configured with credentials for deployment)
- [AWS CDK CLI](https://docs.aws.amazon.com/cdk/v2/guide/cli.html) (`npm install -g aws-cdk`)

### Local Development

The easiest way to start developing is using the provided automation scripts which leverage Docker and LocalStack.

1. **Start the local environment**:
   ```bash
   ./scripts/dev.sh
   ```
   This script will start LocalStack, compile the Go services, deploy the local backend to LocalStack, compile the WebAssembly bridge, and start the Next.js development server.

2. **Access the Application**:
   Open [http://localhost:3000](http://localhost:3000) in your browser.

- *Note: If you modify files in the `core/` directory, the Wasm module will be automatically recompiled by the `air-wasm` Docker container, though you can manually trigger it with `./scripts/internal/build-wasm.sh` if needed.*

## Deployment (AWS Production)

GophDrive includes an automated script for deploying the entire stack to your AWS account.

### 1. Configure Google OAuth
Before deploying, you must create a Google Cloud Project and configure OAuth 2.0 Credentials:
- Go to the [Google Cloud Console](https://console.cloud.google.com).
- Create a project and navigate to **APIs & Services > Credentials**.
- Create an **OAuth client ID** (Web application).
- Set the Authorized Redirect URI to your intended domain's `/api/auth/callback` path (e.g., `https://gophdrive.example.com/api/auth/callback` or the CloudFront URL after deployment).

### 2. Run the Deployment Script
Set your credentials as environment variables and run the script:

```bash
export GOOGLE_CLIENT_ID="your-google-client-id"
export GOOGLE_CLIENT_SECRET="your-google-client-secret"

# Optional: To use a custom domain name
export CUSTOM_DOMAIN_NAME="gophdrive.your-domain.com"
export CERTIFICATE_ARN="arn:aws:acm:us-east-1:123456789012:certificate/uuid"

./scripts/deploy-aws.sh
```

The script will automatically:
1. Manage secure secrets via AWS Systems Manager Parameter Store.
2. Build the Go Lambda backend.
3. Deploy the backend and database infrastructure via AWS CDK.
4. Extract the generated `FRONTEND_URL`.
5. Build the Next.js static frontend using the correct URL context.
6. Deploy the frontend assets to the S3 Bucket and invalidate the CloudFront cache.

---

*See `PROJECT_GUIDE.md` for deeper architectural details and contribution guidelines.*
