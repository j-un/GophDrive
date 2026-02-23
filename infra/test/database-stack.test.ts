import * as cdk from "aws-cdk-lib";
import { Template, Match } from "aws-cdk-lib/assertions";
import { DatabaseStack } from "../lib/database-stack";

describe("DatabaseStack", () => {
  let template: Template;

  beforeAll(() => {
    const app = new cdk.App();
    const stack = new DatabaseStack(app, "TestDatabaseStack");
    template = Template.fromStack(stack);
  });

  test("creates UserTokens DynamoDB table", () => {
    template.hasResourceProperties("AWS::DynamoDB::Table", {
      KeySchema: [
        {
          AttributeName: "user_id",
          KeyType: "HASH",
        },
      ],
      BillingMode: "PAY_PER_REQUEST",
      PointInTimeRecoverySpecification: {
        PointInTimeRecoveryEnabled: true,
      },
    });
  });

  test("creates EditingSessions DynamoDB table", () => {
    template.hasResourceProperties("AWS::DynamoDB::Table", {
      KeySchema: [
        {
          AttributeName: "file_id",
          KeyType: "HASH",
        },
      ],
      BillingMode: "PAY_PER_REQUEST",
      TimeToLiveSpecification: {
        Enabled: true,
        AttributeName: "expires_at",
      },
    });
  });

  test("creates FileStore DynamoDB table", () => {
    template.hasResourceProperties("AWS::DynamoDB::Table", {
      KeySchema: [
        {
          AttributeName: "pk",
          KeyType: "HASH",
        },
      ],
      BillingMode: "PAY_PER_REQUEST",
      TimeToLiveSpecification: {
        Enabled: true,
        AttributeName: "ttl",
      },
    });
  });

  test("UserTokens table has RETAIN removal policy", () => {
    template.hasResource("AWS::DynamoDB::Table", {
      Properties: {
        KeySchema: [{ AttributeName: "user_id", KeyType: "HASH" }],
      },
      DeletionPolicy: "Retain",
      UpdateReplacePolicy: "Retain",
    });
  });

  test("EditingSessions table has DELETE removal policy", () => {
    template.hasResource("AWS::DynamoDB::Table", {
      Properties: {
        KeySchema: [{ AttributeName: "file_id", KeyType: "HASH" }],
      },
      DeletionPolicy: "Delete",
    });
  });

  test("FileStore table has DELETE removal policy", () => {
    template.hasResource("AWS::DynamoDB::Table", {
      Properties: {
        KeySchema: [{ AttributeName: "pk", KeyType: "HASH" }],
      },
      DeletionPolicy: "Delete",
    });
  });

  test("creates exactly 3 DynamoDB tables", () => {
    template.resourceCountIs("AWS::DynamoDB::Table", 3);
  });

  test("outputs table names", () => {
    template.hasOutput("UserTokensTableName", {
      Value: Match.objectLike({ Ref: Match.anyValue() }),
    });
    template.hasOutput("EditingSessionsTableName", {
      Value: Match.objectLike({ Ref: Match.anyValue() }),
    });
    template.hasOutput("FileStoreTableName", {
      Value: Match.objectLike({ Ref: Match.anyValue() }),
    });
  });
});
