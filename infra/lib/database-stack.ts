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

  /** APIKeyHashes table — hashed API keys for programmatic access. */
  public readonly apiKeyHashesTable: dynamodb.Table;

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
    // Point-in-time recovery gives 35 days of any-second restores. At our
    // data volume (a single user, text-only notes) the cost is negligible
    // and it's the cheapest insurance against an accidental delete or a
    // bad migration. EditingSessions is intentionally excluded — it's
    // ephemeral lock state, repopulated by clients within seconds.
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

    // ==========================================================================
    // APIKeyHashes Table
    // --------------------------------------------------------------------------
    // PK: pk (string) — sha256(plaintext key), so plaintext is never stored.
    // GSI: user_id-index (user_id) — for per-user lookup during Issue/Revoke.
    // PITR: enabled — ensures recovery if a bad migration corrupts key records.
    // RemovalPolicy: RETAIN — API keys are user data; losing them locks out CLI.
    // ==========================================================================
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

    // GSI is eventually consistent. Issue guards against races with
    // ConditionExpression on PutItem and TransactWriteItems.
    this.apiKeyHashesTable.addGlobalSecondaryIndex({
      indexName: "user_id-index",
      partitionKey: {
        name: "user_id",
        type: dynamodb.AttributeType.STRING,
      },
      projectionType: dynamodb.ProjectionType.KEYS_ONLY,
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

    new cdk.CfnOutput(this, "APIKeyHashesTableName", {
      value: this.apiKeyHashesTable.tableName,
      description: "DynamoDB table for hashed API keys",
    });
  }
}
