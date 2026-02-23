import * as cdk from "aws-cdk-lib";
import { Template, Match } from "aws-cdk-lib/assertions";
import { SecurityStack } from "../lib/security-stack";

describe("SecurityStack", () => {
  let template: Template;

  beforeAll(() => {
    const app = new cdk.App();
    const stack = new SecurityStack(app, "TestSecurityStack");
    template = Template.fromStack(stack);
  });

  test("creates a KMS key", () => {
    template.resourceCountIs("AWS::KMS::Key", 1);
  });

  test("KMS key has rotation enabled", () => {
    template.hasResourceProperties("AWS::KMS::Key", {
      EnableKeyRotation: true,
    });
  });

  test("KMS key has correct description", () => {
    template.hasResourceProperties("AWS::KMS::Key", {
      Description: Match.stringLikeRegexp("refresh token"),
    });
  });

  test("creates a KMS alias", () => {
    template.hasResourceProperties("AWS::KMS::Alias", {
      AliasName: "alias/gophdrive-token-key",
    });
  });

  test("KMS key has RETAIN removal policy", () => {
    template.hasResource("AWS::KMS::Key", {
      DeletionPolicy: "Retain",
      UpdateReplacePolicy: "Retain",
    });
  });

  test("outputs key ARN", () => {
    template.hasOutput("TokenEncryptionKeyArn", {
      Value: Match.anyValue(),
    });
  });
});
