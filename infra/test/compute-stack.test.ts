import * as cdk from "aws-cdk-lib";
import { Template, Match } from "aws-cdk-lib/assertions";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import * as kms from "aws-cdk-lib/aws-kms";
import { ComputeStack } from "../lib/compute-stack";

describe("ComputeStack", () => {
  let template: Template;

  beforeAll(() => {
    const app = new cdk.App();

    // Create mock dependency stacks
    const depStack = new cdk.Stack(app, "DepStack");

    const userTokensTable = new dynamodb.Table(depStack, "UserTokens", {
      partitionKey: { name: "user_id", type: dynamodb.AttributeType.STRING },
    });
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
    const tokenEncryptionKey = new kms.Key(depStack, "Key");

    const stack = new ComputeStack(app, "TestComputeStack", {
      userTokensTable,
      editingSessionsTable,
      fileStoreTable,
      tokenEncryptionKey,
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
          USER_TOKENS_TABLE: Match.anyValue(),
          EDITING_SESSIONS_TABLE: Match.anyValue(),
          FILE_STORE_TABLE: Match.anyValue(),
          KMS_KEY_ID: Match.anyValue(),
          GOOGLE_CLIENT_SECRET_PARAM: "/gophdrive/google-client-secret",
          JWT_SECRET_PARAM: "/gophdrive/jwt-secret",
          API_GATEWAY_SECRET_PARAM: "/gophdrive/api-gateway-secret",
        }),
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
    // Lambda should have IAM policy allowing DynamoDB access
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
});
