import * as cdk from "aws-cdk-lib";
import * as dynamodb from "aws-cdk-lib/aws-dynamodb";
import { Construct } from "constructs";

/**
 * DatabaseStack
 *
 * Defines the DynamoDB tables for GophDrive:
 * - UserTokens: Stores encrypted OAuth2 refresh tokens per user.
 * - EditingSessions: Manages file-level edit session locks with TTL.
 */
export class DatabaseStack extends cdk.Stack {
  /** UserTokens table — stores encrypted refresh tokens. */
  public readonly userTokensTable: dynamodb.Table;

  /** EditingSessions table — session lock management with TTL. */
  public readonly editingSessionsTable: dynamodb.Table;

  /** FileStore table — storage for Demo Mode files with TTL. */
  public readonly fileStoreTable: dynamodb.Table;

  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // ==========================================================================
    // UserTokens Table
    // --------------------------------------------------------------------------
    // PK: user_id (string)
    // Attributes: encrypted_refresh_token, updated_at
    // Point-in-time recovery enabled for safety.
    // ==========================================================================
    this.userTokensTable = new dynamodb.Table(this, "UserTokensTable", {
      partitionKey: {
        name: "user_id",
        type: dynamodb.AttributeType.STRING,
      },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      pointInTimeRecoverySpecification: {
        pointInTimeRecoveryEnabled: true,
      },
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });

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
    // FileStore Table (for Demo Mode)
    // --------------------------------------------------------------------------
    // PK: pk (string)
    // Attributes: user_id, ttl
    // TTL automatically removes expired demo files.
    // ==========================================================================
    this.fileStoreTable = new dynamodb.Table(this, "FileStoreTable", {
      partitionKey: {
        name: "pk",
        type: dynamodb.AttributeType.STRING,
      },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      timeToLiveAttribute: "ttl",
      removalPolicy: cdk.RemovalPolicy.DESTROY, // Demo data is ephemeral
    });

    // ==========================================================================
    // Outputs
    // ==========================================================================
    new cdk.CfnOutput(this, "UserTokensTableName", {
      value: this.userTokensTable.tableName,
      description: "DynamoDB table for OAuth2 refresh tokens",
    });

    new cdk.CfnOutput(this, "EditingSessionsTableName", {
      value: this.editingSessionsTable.tableName,
      description: "DynamoDB table for file editing session locks",
    });

    new cdk.CfnOutput(this, "FileStoreTableName", {
      value: this.fileStoreTable.tableName,
      description: "DynamoDB table for demo mode files",
    });
  }
}
