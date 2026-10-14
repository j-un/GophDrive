import * as cdk from "aws-cdk-lib";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import { Construct } from "constructs";

export class DatabaseStack extends cdk.Stack {
  public readonly editingSessionsTable: dynamodb.Table;

  public readonly fileStoreTable: dynamodb.Table;

  public readonly apiKeyHashesTable: dynamodb.Table;

  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    this.editingSessionsTable = new dynamodb.Table(
      this,
      "EditingSessionsTable",
      {
        partitionKey: {
          name: "file_id",
          type: dynamodb.AttributeType.STRING,
        },
        billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
        timeToLiveAttribute: "expires_at",
        removalPolicy: cdk.RemovalPolicy.DESTROY,
      },
    );

    this.fileStoreTable = new dynamodb.Table(this, "FileStoreTable", {
      partitionKey: {
        name: "pk",
        type: dynamodb.AttributeType.STRING,
      },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      timeToLiveAttribute: "ttl",
      removalPolicy: cdk.RemovalPolicy.RETAIN,
      pointInTimeRecoverySpecification: {
        pointInTimeRecoveryEnabled: true,
      },
    });

    this.apiKeyHashesTable = new dynamodb.Table(this, "APIKeyHashesTable", {
      partitionKey: {
        name: "pk",
        type: dynamodb.AttributeType.STRING,
      },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: cdk.RemovalPolicy.RETAIN,
      pointInTimeRecoverySpecification: {
        pointInTimeRecoveryEnabled: true,
      },
    });

    this.apiKeyHashesTable.addGlobalSecondaryIndex({
      indexName: "user_id-index",
      partitionKey: {
        name: "user_id",
        type: dynamodb.AttributeType.STRING,
      },
      projectionType: dynamodb.ProjectionType.KEYS_ONLY,
    });

    new cdk.CfnOutput(this, "EditingSessionsTableName", {
      value: this.editingSessionsTable.tableName,
      description: "DynamoDB table for file editing session locks",
    });

    new cdk.CfnOutput(this, "FileStoreTableName", {
      value: this.fileStoreTable.tableName,
      description: "DynamoDB table for notes and folders",
    });

    new cdk.CfnOutput(this, "APIKeyHashesTableName", {
      value: this.apiKeyHashesTable.tableName,
      description: "DynamoDB table for hashed API keys",
    });
  }
}
