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

  test("EditingSessions table has DELETE removal policy", () => {
    template.hasResource("AWS::DynamoDB::Table", {
      Properties: {
        KeySchema: [{ AttributeName: "file_id", KeyType: "HASH" }],
      },
      DeletionPolicy: "Delete",
    });
  });

  test("FileStore table has RETAIN removal policy", () => {
    template.hasResource("AWS::DynamoDB::Table", {
      Properties: {
        KeySchema: [{ AttributeName: "pk", KeyType: "HASH" }],
      },
      DeletionPolicy: "Retain",
      UpdateReplacePolicy: "Retain",
    });
  });

  test("FileStore has point-in-time recovery enabled", () => {
    template.hasResourceProperties("AWS::DynamoDB::Table", {
      KeySchema: [{ AttributeName: "pk", KeyType: "HASH" }],
      PointInTimeRecoverySpecification: {
        PointInTimeRecoveryEnabled: true,
      },
    });
  });

  test("EditingSessions does not enable PITR (ephemeral lock state)", () => {
    template.hasResourceProperties("AWS::DynamoDB::Table", {
      KeySchema: [{ AttributeName: "file_id", KeyType: "HASH" }],
      PointInTimeRecoverySpecification: Match.absent(),
    });
  });

  test("creates exactly 3 DynamoDB tables", () => {
    template.resourceCountIs("AWS::DynamoDB::Table", 3);
  });

  test("creates APIKeyHashes table with user_id GSI", () => {
    template.hasResourceProperties("AWS::DynamoDB::Table", {
      BillingMode: "PAY_PER_REQUEST",
      GlobalSecondaryIndexes: Match.arrayWith([
        Match.objectLike({
          IndexName: "user_id-index",
          KeySchema: [{ AttributeName: "user_id", KeyType: "HASH" }],
          Projection: { ProjectionType: "KEYS_ONLY" },
        }),
      ]),
      PointInTimeRecoverySpecification: {
        PointInTimeRecoveryEnabled: true,
      },
    });
  });

  test("outputs table names", () => {
    template.hasOutput("EditingSessionsTableName", {
      Value: Match.objectLike({ Ref: Match.anyValue() }),
    });
    template.hasOutput("FileStoreTableName", {
      Value: Match.objectLike({ Ref: Match.anyValue() }),
    });
    template.hasOutput("APIKeyHashesTableName", {
      Value: Match.objectLike({ Ref: Match.anyValue() }),
    });
  });
});
