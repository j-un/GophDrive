GophDrive is a serverless Markdown note-taking application that uses Google Drive for storage. It is designed to be highly secure, private, and easy to deploy on AWS. The system is built with extensibility in mind, allowing for future support of alternative storage providers beyond Google Drive.

### Storage Abstraction

- **Adapter Pattern**: All external storage interactions are centralized in `backend/internal/adapter`.
- **Decoupled Logic**: Business logic is designed to interact only with storage interfaces. It remains entirely agnostic of specific implementation details (e.g., Google Drive API vs. Local Filesystem).
- **Extensibility**: To add a new storage provider, one only needs to implement the `StorageAdapter` interface within a new package under the `adapter` directory.

## Project Overview

- **Storage**: Notes are stored as Markdown files in the user's Google Drive. The application only accesses a specific root folder chosen by the user.
  - **Demo Mode**: Uses DynamoDB (`FileStore` table) via `MemoryAdapter` to provide temporary storage for demo users (`demo-user-*`) with 60-minute TTL.
- **Backend**: Go-based Lambda functions running on AWS Lambda + API Gateway.
- **Frontend**: Next.js (App Router) Single Page Application (SPA), exported as a static site and hosted on S3 + CloudFront.
- **Core Logic**: Shared Go logic (`core/`) is compiled to WebAssembly (Wasm) and executed in the browser for client-side operations (like previewing and conflict resolution).
- **Authentication**: Google OAuth2 for production; a "Demo Mode" (accessible via `/auth/demo-login`) allows trying the app without an account.

## Project Structure

```text
GophDrive/
├── backend/            # Go Backend (AWS Lambda)
│   ├── cmd/            # Main entry points for Lambda
│   ├── internal/       # Internal packages (adapters, handlers, auth)
│   └── api/            # (Build artifact) Compiled backend binary
├── core/               # Shared Go logic (Backend & Wasm)
│   ├── bridge/         # Wasm JS bridge logic
│   ├── markdown/       # Markdown processing
│   └── sync/           # Conflict detection and sync logic
├── frontend/           # Next.js Frontend
│   ├── src/app/        # App Router pages
│   ├── src/components/ # UI Components
│   ├── src/context/    # React Context (Auth)
│   └── public/         # Static assets (including wasm_exec.js)
├── infra/              # Infrastructure as Code (AWS CDK)
│   └── lib/            # CDK Stacks (Frontend, Backend, Database)
├── scripts/            # Automation scripts
└── docker-compose.yml  # Local development environment (LocalStack)
```

## Directory Roles & Languages

| Directory  | Language    | Role                                                                          |
| :--------- | :---------- | :---------------------------------------------------------------------------- |
| `backend`  | Go          | REST API, Google Drive integration, DynamoDB persistence for sessions.        |
| `frontend` | TypeScript  | UI/UX, calling the backend API, executing Wasm core logic.                    |
| `core`     | Go          | Business logic shared between backend and frontend (Wasm target).             |
| `infra`    | TypeScript  | AWS Resource definitions (CDK). Now includes `FileStore` table for Demo Mode. |
| `scripts`  | Shell (zsh) | Build, local deployment, and AWS deployment automation.                       |

## Key Development & Operation Commands

### 1. Environment Setup

Initial setup of the development environment:

```bash
./scripts/dev.sh
```

### 2. Building Core logic (Wasm)

Must be run whenever files in `core/` are modified:

```bash
./scripts/internal/build-wasm.sh
```

### 3. Local Development (LocalStack)

Start the local environment (DynamoDB, KMS mocks via LocalStack):

```bash
docker-compose up -d
./scripts/internal/deploy-local.sh
```

The frontend can then be run via `npm run dev` in the `frontend` directory.

### 4. Deployment to AWS

Full deployment to the production AWS account:

```bash
./scripts/deploy-aws.sh
```

## AI Agent Handover Notes

- **Wasm Execution**: The frontend relies on `wasm_exec.js` and `initWasm()` for core features. If Wasm logic changes, ensure `scripts/internal/build-wasm.sh` is called.
- **SPA Routing**: Production is hosted on S3/CloudFront. SPA-style direct navigation (e.g., `/notes/`) is handled by a CloudFront Function that appends `index.html` to directory requests.
- **Conflict Management**: GophDrive uses a session-based locking mechanism (`internal/session`) to prevent concurrent edit conflicts.
- **Demo Mode**: Demo users use a hybrid storage provider in the backend. Data is ephemeral (DynamoDB TTL).
