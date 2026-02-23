import * as cdk from "aws-cdk-lib";
import * as kms from "aws-cdk-lib/aws-kms";
import { Construct } from "constructs";

/**
 * SecurityStack
 *
 * Defines security-related resources:
 * - KMS Key for encrypting/decrypting OAuth2 refresh tokens.
 */
export class SecurityStack extends cdk.Stack {
  /** KMS key used to encrypt refresh tokens stored in DynamoDB. */
  public readonly tokenEncryptionKey: kms.Key;

  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // ==========================================================================
    // KMS Key — Refresh Token Encryption
    // ==========================================================================
    this.tokenEncryptionKey = new kms.Key(this, "TokenEncryptionKey", {
      alias: "alias/gophdrive-token-key",
      description:
        "Encrypts OAuth2 refresh tokens stored in the UserTokens DynamoDB table",
      enableKeyRotation: true,
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });

    // ==========================================================================
    // Outputs
    // ==========================================================================
    new cdk.CfnOutput(this, "TokenEncryptionKeyArn", {
      value: this.tokenEncryptionKey.keyArn,
      description: "ARN of the KMS key for token encryption",
    });
  }
}
