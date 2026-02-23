import * as cdk from "aws-cdk-lib";
import { Construct } from "constructs";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as apigateway from "aws-cdk-lib/aws-apigateway";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as iam from "aws-cdk-lib/aws-iam";
import * as kms from "aws-cdk-lib/aws-kms";
import * as path from "path";
import { execSync } from "child_process";

interface ComputeStackProps extends cdk.StackProps {
  userTokensTable: dynamodb.Table;
  editingSessionsTable: dynamodb.Table;
  fileStoreTable: dynamodb.Table;
  tokenEncryptionKey: kms.Key;
}

export class ComputeStack extends cdk.Stack {
  public readonly api: apigateway.RestApi;

  constructor(scope: Construct, id: string, props: ComputeStackProps) {
    super(scope, id, props);

    // Lambda Function
    const backendFunction = new lambda.Function(this, "BackendFunction", {
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: "bootstrap",
      architecture: lambda.Architecture.ARM_64,
      code: lambda.Code.fromAsset(path.join(__dirname, "../../backend"), {
        bundling: {
          image: lambda.Runtime.PROVIDED_AL2023.bundlingImage,
          command: [
            "bash",
            "-c",
            "GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o /asset-output/bootstrap ./cmd/api",
          ],
          local: {
            tryBundle(outputDir: string) {
              try {
                execSync("go version", { stdio: "ignore" });
                const buildCmd = `GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o ${path.join(outputDir, "bootstrap")} ./cmd/api`;
                execSync(buildCmd, {
                  cwd: path.join(__dirname, "../../backend"),
                  stdio: "inherit",
                });
                return true;
              } catch (error) {
                console.log("Local bundling failed, using Docker:", error);
                return false;
              }
            },
          },
        },
      }),
      environment: {
        USER_TOKENS_TABLE: props.userTokensTable.tableName,
        EDITING_SESSIONS_TABLE: props.editingSessionsTable.tableName,
        FILE_STORE_TABLE: props.fileStoreTable.tableName,
        KMS_KEY_ID: props.tokenEncryptionKey.keyId,
        GOOGLE_CLIENT_ID: process.env.GOOGLE_CLIENT_ID || "",
        GOOGLE_CLIENT_SECRET_PARAM: "/gophdrive/google-client-secret",
        JWT_SECRET_PARAM: "/gophdrive/jwt-secret",
        API_GATEWAY_SECRET_PARAM: "/gophdrive/api-gateway-secret",
        FRONTEND_URL: process.env.FRONTEND_URL || "http://localhost:3000",
        GOOGLE_REDIRECT_URL: `${process.env.FRONTEND_URL || "http://localhost:3000"}/api/auth/callback`,
      },
      timeout: cdk.Duration.seconds(30),
      memorySize: 128,
    });

    // Grant Permissions
    props.userTokensTable.grantReadWriteData(backendFunction);
    props.editingSessionsTable.grantReadWriteData(backendFunction);
    props.fileStoreTable.grantReadWriteData(backendFunction);
    props.tokenEncryptionKey.grantEncryptDecrypt(backendFunction);

    // Grant SSM Parameter Store read access for secrets
    backendFunction.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: ["ssm:GetParameter"],
        resources: [
          `arn:aws:ssm:${this.region}:${this.account}:parameter/gophdrive/*`,
        ],
      }),
    );

    // API Gateway
    this.api = new apigateway.RestApi(this, "GophDriveAPI", {
      restApiName: "GophDrive API",
      description: "API for GophDrive Backend",
      defaultCorsPreflightOptions: {
        allowOrigins: apigateway.Cors.ALL_ORIGINS,
        allowMethods: apigateway.Cors.ALL_METHODS,
        allowHeaders: ["Content-Type", "Authorization"],
      },
    });

    const integration = new apigateway.LambdaIntegration(backendFunction);
    this.api.root.addProxy({
      defaultIntegration: integration,
    });

    // Outputs
    new cdk.CfnOutput(this, "ApiUrl", {
      value: this.api.url,
      description: "API Gateway URL",
    });
  }
}
