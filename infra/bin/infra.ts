#!/usr/bin/env node
import "source-map-support/register";
import * as cdk from "aws-cdk-lib";
import { DatabaseStack } from "../lib/database-stack";
import { SecurityStack } from "../lib/security-stack";

import { ComputeStack } from "../lib/compute-stack";
import { FrontendStack } from "../lib/frontend-stack";

const app = new cdk.App();

const env: cdk.Environment = {
  account: process.env.CDK_DEFAULT_ACCOUNT || "000000000000",
  region: process.env.CDK_DEFAULT_REGION || "ap-northeast-1",
};

// ==============================================================================
// Stack instantiation
// ==============================================================================
const securityStack = new SecurityStack(app, "GophDriveSecurityStack", {
  env,
  description: "GophDrive - Security resources (KMS)",
});

const databaseStack = new DatabaseStack(app, "GophDriveDatabaseStack", {
  env,
  description: "GophDrive - DynamoDB tables",
});

const computeStack = new ComputeStack(app, "GophDriveComputeStack", {
  env,
  description: "GophDrive - Backend (Lambda + API Gateway)",
  userTokensTable: databaseStack.userTokensTable,
  editingSessionsTable: databaseStack.editingSessionsTable,
  fileStoreTable: databaseStack.fileStoreTable,
  tokenEncryptionKey: securityStack.tokenEncryptionKey,
});

computeStack.addDependency(databaseStack);
computeStack.addDependency(securityStack);

new FrontendStack(app, "GophDriveFrontendStack", {
  env,
  description: "GophDrive - Frontend (S3 + CloudFront)",
  apiGatewayDomain: `${computeStack.api.restApiId}.execute-api.${computeStack.region}.amazonaws.com`,
});
// Frontend doesn't explicitly depend on backend stack to be created,
// but logically we need the API URL first for the build.
// However, in this setup, we might deploy concurrently.

app.synth();
