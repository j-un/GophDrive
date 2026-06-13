import * as cdk from "aws-cdk-lib";
import { Template, Match } from "aws-cdk-lib/assertions";
import { FrontendStack, STRICT_CSP } from "../lib/frontend-stack";

describe("FrontendStack", () => {
  let template: Template;

  beforeAll(() => {
    const app = new cdk.App();
    const stack = new FrontendStack(app, "TestFrontendStack", {
      apiGatewayDomain: "abc123.execute-api.ap-northeast-1.amazonaws.com",
    });
    template = Template.fromStack(stack);
  });

  test("creates an S3 bucket with public access blocked", () => {
    template.hasResourceProperties("AWS::S3::Bucket", {
      PublicAccessBlockConfiguration: {
        BlockPublicAcls: true,
        BlockPublicPolicy: true,
        IgnorePublicAcls: true,
        RestrictPublicBuckets: true,
      },
    });
  });

  test("creates a CloudFront distribution", () => {
    template.resourceCountIs("AWS::CloudFront::Distribution", 1);
  });

  test("CloudFront has HTTPS redirect", () => {
    template.hasResourceProperties("AWS::CloudFront::Distribution", {
      DistributionConfig: Match.objectLike({
        DefaultCacheBehavior: Match.objectLike({
          ViewerProtocolPolicy: "redirect-to-https",
        }),
      }),
    });
  });

  test("CloudFront has no custom error responses that would mask API errors", () => {
    template.hasResourceProperties("AWS::CloudFront::Distribution", {
      DistributionConfig: Match.not(
        Match.objectLike({
          CustomErrorResponses: Match.anyValue(),
        }),
      ),
    });
  });

  test("creates an Origin Access Control for S3", () => {
    template.hasResourceProperties("AWS::CloudFront::OriginAccessControl", {
      OriginAccessControlConfig: Match.objectLike({
        OriginAccessControlOriginType: "s3",
        SigningBehavior: "always",
        SigningProtocol: "sigv4",
      }),
    });
  });

  test("outputs frontend URL", () => {
    template.hasOutput("FrontendUrl", {
      Value: Match.anyValue(),
    });
  });

  test("attaches a strict CSP to the CloudFront ResponseHeadersPolicy", () => {
    template.hasResourceProperties("AWS::CloudFront::ResponseHeadersPolicy", {
      ResponseHeadersPolicyConfig: Match.objectLike({
        SecurityHeadersConfig: Match.objectLike({
          ContentSecurityPolicy: {
            ContentSecurityPolicy: STRICT_CSP,
            Override: true,
          },
        }),
      }),
    });
  });

  test("CSP script-src does not allow unsafe-eval", () => {
    expect(STRICT_CSP).not.toMatch(/script-src[^;]*'unsafe-eval'(?!-)/);
  });
});
