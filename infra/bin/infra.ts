#!/usr/bin/env node
import "source-map-support/register";
import * as cdk from "aws-cdk-lib";
import { DatabaseStack } from "../lib/database-stack";

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
const databaseStack = new DatabaseStack(app, "GophDriveDatabaseStack", {
  env,
  description: "GophDrive - DynamoDB tables",
});

const computeStack = new ComputeStack(app, "GophDriveComputeStack", {
  env,
  description: "GophDrive - Backend (Lambda + API Gateway)",
  editingSessionsTable: databaseStack.editingSessionsTable,
  fileStoreTable: databaseStack.fileStoreTable,
});

computeStack.addDependency(databaseStack);

new FrontendStack(app, "GophDriveFrontendStack", {
  env,
  description: "GophDrive - Frontend (S3 + CloudFront)",
  apiGatewayDomain: `${computeStack.api.restApiId}.execute-api.${computeStack.region}.amazonaws.com`,
});

app.synth();
