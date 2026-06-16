import * as cdk from "aws-cdk-lib";
import { Construct } from "constructs";
import * as lambda from "aws-cdk-lib/aws-lambda";
import * as apigateway from "aws-cdk-lib/aws-apigateway";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as iam from "aws-cdk-lib/aws-iam";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as path from "path";
import { execSync } from "child_process";

interface ComputeStackProps extends cdk.StackProps {
  editingSessionsTable: dynamodb.Table;
  fileStoreTable: dynamodb.Table;
  apiKeyHashesTable: dynamodb.Table;
}

export class ComputeStack extends cdk.Stack {
  public readonly api: apigateway.RestApi;
  public readonly bodyStoreBucket: s3.Bucket;

  constructor(scope: Construct, id: string, props: ComputeStackProps) {
    super(scope, id, props);

    // ==========================================================================
    // BodyStore S3 Bucket
    // --------------------------------------------------------------------------
    // Reserved for the future spillover path: bodies that exceed the
    // DynamoDB inline budget will be uploaded here and FileStore items will
    // hold an `body_s3_key` pointer instead of inline content. The bucket
    // is provisioned now so the migration to that mode is just a code change.
    //
    // RETAIN policy because the data, once written, is the user's notes.
    // ==========================================================================
    this.bodyStoreBucket = new s3.Bucket(this, "BodyStoreBucket", {
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      enforceSSL: true,
      versioned: false,
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });

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
        EDITING_SESSIONS_TABLE: props.editingSessionsTable.tableName,
        FILE_STORE_TABLE: props.fileStoreTable.tableName,
        BODY_STORE_BUCKET: this.bodyStoreBucket.bucketName,
        GOOGLE_CLIENT_ID: process.env.GOOGLE_CLIENT_ID || "",
        GOOGLE_CLIENT_SECRET_PARAM: "/gophdrive/google-client-secret",
        JWT_SECRET_PARAM: "/gophdrive/jwt-secret",
        API_GATEWAY_SECRET_PARAM: "/gophdrive/api-gateway-secret",
        API_KEY_HASHES_TABLE: props.apiKeyHashesTable.tableName,
        FRONTEND_URL: process.env.FRONTEND_URL || "http://localhost:5173",
        GOOGLE_REDIRECT_URL: `${process.env.FRONTEND_URL || "http://localhost:5173"}/api/auth/callback`,
        ALLOWED_EMAILS: process.env.ALLOWED_EMAILS || "",
      },
      timeout: cdk.Duration.seconds(30),
      memorySize: 128,
    });

    // Grant Permissions
    props.editingSessionsTable.grantReadWriteData(backendFunction);
    props.fileStoreTable.grantReadWriteData(backendFunction);
    backendFunction.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:DeleteItem",
          "dynamodb:TransactWriteItems",
        ],
        resources: [props.apiKeyHashesTable.tableArn],
      }),
    );
    backendFunction.addToRolePolicy(
      new iam.PolicyStatement({
        effect: iam.Effect.ALLOW,
        actions: ["dynamodb:Query"],
        resources: [`${props.apiKeyHashesTable.tableArn}/index/user_id-index`],
      }),
    );
    this.bodyStoreBucket.grantReadWrite(backendFunction);

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
      binaryMediaTypes: ["application/zip"],
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

    new cdk.CfnOutput(this, "BodyStoreBucketName", {
      value: this.bodyStoreBucket.bucketName,
      description: "S3 bucket reserved for spillover note bodies",
    });
  }
}
