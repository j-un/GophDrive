# Privacy Policy for [Your Service Name]

> **Note for hosters**: This is a template. Replace all `[placeholder]` values with your own information. You are responsible for ensuring this policy complies with applicable laws in your jurisdiction (e.g., GDPR, CCPA).

**Last Updated**: [Date]

## Introduction

[Your Service Name] is a self-hosted instance of [GophDrive](https://github.com/j-un/GophDrive), a serverless Markdown note-taking application. This privacy policy explains how your data is handled when you use this service.

## Data We Collect

### Google Account Information (via OAuth 2.0)

When you sign in with Google, we receive:

- **Email address** — Used to identify your account
- **Basic profile information** (name, profile picture) — Used for display purposes
- **Google Drive access token** — Used to read and write files in your designated Google Drive folder

### Data Stored on the Server

| Data | Storage Location | Purpose | Retention |
|:-----|:-----------------|:--------|:----------|
| OAuth tokens (encrypted) | Amazon DynamoDB | Accessing your Google Drive on your behalf | Until you revoke access |
| Session information | Amazon DynamoDB | Maintaining your login session | Session duration |
| JWT tokens | Browser cookie | Authentication | Session duration |

### Data Stored in Your Google Drive

Your notes are stored as Markdown files directly in a folder you designate in your own Google Drive. The application **does not** copy or store your note content on its servers.

## Google API Scopes

This application requests the following Google API scopes:

- `openid` — Verify your identity
- `email` — Access your email address for account identification
- `profile` — Access your basic profile information
- `https://www.googleapis.com/auth/drive.file` — Read and write only the files created by or opened with this application in your Google Drive

> [!IMPORTANT]
> This application does **not** request access to your entire Google Drive. It only accesses files within the specific folder you designate during setup.

## How We Use Your Data

- To authenticate you and maintain your session
- To read and write your notes in your Google Drive
- To provide conflict detection for concurrent edits

We do **not**:
- Sell your data to third parties
- Use your data for advertising
- Share your data with third parties (except Google, as required for Drive access)

## Data Security

- OAuth tokens are encrypted at rest using [AWS KMS / your encryption method]
- All communications are encrypted in transit via HTTPS (TLS 1.2+)
- [Add any additional security measures specific to your deployment]

## Your Rights

You can:
- **Revoke access** at any time by removing the app from your [Google Account permissions](https://myaccount.google.com/permissions)
- **Delete your data** by removing the designated folder from your Google Drive
- **Request account deletion** by contacting [your contact information]

## Third-Party Services

This application relies on:
- **Google APIs** — For authentication and Drive storage ([Google Privacy Policy](https://policies.google.com/privacy))
- **Amazon Web Services** — For hosting and infrastructure ([AWS Privacy Policy](https://aws.amazon.com/privacy/))

## Changes to This Policy

We may update this privacy policy from time to time. Changes will be posted at this URL with an updated "Last Updated" date.

## Contact

If you have questions about this privacy policy, please contact:
[Your contact information]
