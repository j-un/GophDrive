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

Google is used **solely as an identity provider** for sign-in. GophDrive does not request access to Google Drive or any other Google service beyond your identity.

### Data Stored on the Server

| Data | Storage Location | Purpose | Retention |
|:-----|:-----------------|:--------|:----------|
| Notes and folders | Amazon DynamoDB (FileStore) | Storing your notes and folder structure | Until you delete them (demo accounts: 60 minutes) |
| Edit-session locks | Amazon DynamoDB (EditingSessions) | Preventing conflicting concurrent edits | Short-lived TTL |
| JWT session token | Browser (cookie/localStorage) | Authentication | Session duration |

### Where Your Notes Are Stored

Your notes are stored **server-side in Amazon DynamoDB**, inside the operator's AWS account. Unlike some notes applications, GophDrive is a self-hosted service — your data resides in the infrastructure of the person or organisation running this instance, not in your own Google Drive or any third-party storage.

> [!IMPORTANT]
> If you are the **operator** (self-hoster), your users' notes live in your AWS account's DynamoDB table (`FileStore`). Point-in-time recovery (35-day any-second restore) is enabled by default.

## Google API Scopes

This application requests only the following Google API scopes:

- `openid` — Verify your identity
- `email` — Access your email address for account identification
- `profile` — Access your basic profile information

GophDrive does **not** request any Google Drive, Gmail, or other file-access scope. It cannot read, write, or modify any files in your Google account.

## How We Use Your Data

- To authenticate you and maintain your session
- To store, retrieve, and manage your notes in DynamoDB
- To provide conflict detection for concurrent edits

We do **not**:
- Sell your data to third parties
- Use your data for advertising
- Share your data with third parties (Google is used only to verify your identity at sign-in)

## Data Security

- All data is encrypted at rest via Amazon DynamoDB's default encryption
- All communications are encrypted in transit via HTTPS (TLS 1.2+)
- [Add any additional security measures specific to your deployment]

## Your Rights

You can:
- **Delete your notes** at any time from within the application
- **Export your data** as a ZIP archive via the Settings page
- **Revoke sign-in access** at any time by removing the app from your [Google Account permissions](https://myaccount.google.com/permissions) (this does not delete stored notes)
- **Request account and data deletion** by contacting [your contact information]

## Third-Party Services

This application relies on:
- **Google APIs** — For authentication (sign-in) only ([Google Privacy Policy](https://policies.google.com/privacy))
- **Amazon Web Services** — For hosting, infrastructure, and note storage ([AWS Privacy Policy](https://aws.amazon.com/privacy/))

## Changes to This Policy

We may update this privacy policy from time to time. Changes will be posted at this URL with an updated "Last Updated" date.

## Contact

If you have questions about this privacy policy, please contact:
[Your contact information]
