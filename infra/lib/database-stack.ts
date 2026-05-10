import * as cdk from "aws-cdk-lib";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import { Construct } from "constructs";

/**
 * DatabaseStack
 *
 * Defines the DynamoDB tables for GophDrive:
 * - EditingSessions: file-level edit session locks with TTL.
 * - FileStore: notes and folders, plus ephemeral demo-user data with TTL.
 */
export class DatabaseStack extends cdk.Stack {
  /** EditingSessions table — session lock management with TTL. */
  public readonly editingSessionsTable: dynamodb.Table;

  /** FileStore table — primary storage for notes and folders. */
  public readonly fileStoreTable: dynamodb.Table;

  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // ==========================================================================
    // EditingSessions Table
    // --------------------------------------------------------------------------
    // PK: file_id (string)
    // Attributes: user_id, expires_at (TTL)
    // TTL automatically removes expired session locks.
    // ==========================================================================
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

    // ==========================================================================
    // FileStore Table
    // --------------------------------------------------------------------------
    // PK: pk (string) — corresponds to the file/folder ID.
    // Attributes: user_id, ttl (only set for demo users)
    // ==========================================================================
    this.fileStoreTable = new dynamodb.Table(this, "FileStoreTable", {
      partitionKey: {
        name: "pk",
        type: dynamodb.AttributeType.STRING,
      },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      timeToLiveAttribute: "ttl",
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });

    // ==========================================================================
    // Outputs
    // ==========================================================================
    new cdk.CfnOutput(this, "EditingSessionsTableName", {
      value: this.editingSessionsTable.tableName,
      description: "DynamoDB table for file editing session locks",
    });

    new cdk.CfnOutput(this, "FileStoreTableName", {
      value: this.fileStoreTable.tableName,
      description: "DynamoDB table for notes and folders",
    });
  }
}
