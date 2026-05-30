import * as cdk from "aws-cdk-lib";
import { Template, Match } from "aws-cdk-lib/assertions";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import { ComputeStack } from "../lib/compute-stack";

describe("ComputeStack", () => {
  let template: Template;

  beforeAll(() => {
    const app = new cdk.App();

    // Create mock dependency stacks
    const depStack = new cdk.Stack(app, "DepStack");

    const editingSessionsTable = new dynamodb.Table(
      depStack,
      "EditingSessions",
      {
        partitionKey: { name: "file_id", type: dynamodb.AttributeType.STRING },
      },
    );
    const fileStoreTable = new dynamodb.Table(depStack, "FileStore", {
      partitionKey: { name: "pk", type: dynamodb.AttributeType.STRING },
    });
    const apiKeyHashesTable = new dynamodb.Table(depStack, "APIKeyHashes", {
      partitionKey: { name: "pk", type: dynamodb.AttributeType.STRING },
    });

    const stack = new ComputeStack(app, "TestComputeStack", {
      editingSessionsTable,
      fileStoreTable,
      apiKeyHashesTable,
    });
    template = Template.fromStack(stack);
  });

  test("creates a Lambda function with ARM64 architecture", () => {
    template.hasResourceProperties("AWS::Lambda::Function", {
      Architectures: ["arm64"],
      Runtime: "provided.al2023",
      Handler: "bootstrap",
      Timeout: 30,
      MemorySize: 128,
    });
  });

  test("Lambda has required environment variables with SSM param names", () => {
    template.hasResourceProperties("AWS::Lambda::Function", {
      Environment: {
        Variables: Match.objectLike({
          EDITING_SESSIONS_TABLE: Match.anyValue(),
          FILE_STORE_TABLE: Match.anyValue(),
          API_KEY_HASHES_TABLE: Match.anyValue(),
          BODY_STORE_BUCKET: Match.anyValue(),
          GOOGLE_CLIENT_SECRET_PARAM: "/gophdrive/google-client-secret",
          JWT_SECRET_PARAM: "/gophdrive/jwt-secret",
          API_GATEWAY_SECRET_PARAM: "/gophdrive/api-gateway-secret",
          ALLOWED_EMAILS: Match.anyValue(),
        }),
      },
    });
  });

  test("Lambda env does not include deprecated AGENT_API_KEY_PARAM", () => {
    template.hasResourceProperties("AWS::Lambda::Function", {
      Environment: {
        Variables: Match.not(
          Match.objectLike({ AGENT_API_KEY_PARAM: Match.anyValue() }),
        ),
      },
    });
  });

  test("creates BodyStore S3 bucket with public access blocked", () => {
    template.resourceCountIs("AWS::S3::Bucket", 1);
    template.hasResourceProperties("AWS::S3::Bucket", {
      PublicAccessBlockConfiguration: {
        BlockPublicAcls: true,
        BlockPublicPolicy: true,
        IgnorePublicAcls: true,
        RestrictPublicBuckets: true,
      },
      BucketEncryption: {
        ServerSideEncryptionConfiguration: Match.arrayWith([
          Match.objectLike({
            ServerSideEncryptionByDefault: { SSEAlgorithm: "AES256" },
          }),
        ]),
      },
    });
  });

  test("BodyStore bucket has RETAIN removal policy", () => {
    template.hasResource("AWS::S3::Bucket", {
      DeletionPolicy: "Retain",
      UpdateReplacePolicy: "Retain",
    });
  });

  test("Lambda has read/write IAM access to BodyStore bucket", () => {
    template.hasResourceProperties("AWS::IAM::Policy", {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Action: Match.arrayWith(["s3:GetObject*", "s3:PutObject"]),
            Effect: "Allow",
          }),
        ]),
      },
    });
  });

  test("Lambda env does not include legacy KMS / UserTokens vars", () => {
    template.hasResourceProperties("AWS::Lambda::Function", {
      Environment: {
        Variables: Match.not(
          Match.objectLike({ KMS_KEY_ID: Match.anyValue() }),
        ),
      },
    });
    template.hasResourceProperties("AWS::Lambda::Function", {
      Environment: {
        Variables: Match.not(
          Match.objectLike({ USER_TOKENS_TABLE: Match.anyValue() }),
        ),
      },
    });
  });

  test("Lambda has SSM GetParameter policy", () => {
    template.hasResourceProperties("AWS::IAM::Policy", {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Action: "ssm:GetParameter",
            Effect: "Allow",
          }),
        ]),
      },
    });
  });

  test("creates an API Gateway REST API", () => {
    template.resourceCountIs("AWS::ApiGateway::RestApi", 1);
    template.hasResourceProperties("AWS::ApiGateway::RestApi", {
      Name: "GophDrive API",
    });
  });

  test("API Gateway has CORS configuration", () => {
    // CORS preflight creates an OPTIONS method
    template.hasResourceProperties("AWS::ApiGateway::Method", {
      HttpMethod: "OPTIONS",
    });
  });

  test("grants Lambda read/write access to DynamoDB tables", () => {
    template.hasResourceProperties("AWS::IAM::Policy", {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Action: Match.arrayWith(["dynamodb:BatchGetItem"]),
            Effect: "Allow",
          }),
        ]),
      },
    });
  });

  test("outputs API URL", () => {
    template.hasOutput("ApiUrl", {
      Value: Match.anyValue(),
    });
  });

  test("outputs BodyStore bucket name", () => {
    template.hasOutput("BodyStoreBucketName", {
      Value: Match.anyValue(),
    });
  });
});
