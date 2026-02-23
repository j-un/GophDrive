import * as cdk from "aws-cdk-lib";
import { Construct } from "constructs";
import * as s3 from "aws-cdk-lib/aws-s3";
import * as cloudfront from "aws-cdk-lib/aws-cloudfront";
import * as origins from "aws-cdk-lib/aws-cloudfront-origins";
import * as acm from "aws-cdk-lib/aws-certificatemanager";
// import * as path from "path";

interface FrontendStackProps extends cdk.StackProps {
  apiGatewayDomain: string;
}

export class FrontendStack extends cdk.Stack {
  public readonly distribution: cloudfront.Distribution;

  constructor(scope: Construct, id: string, props: FrontendStackProps) {
    super(scope, id, props);

    // S3 Bucket
    const bucket = new s3.Bucket(this, "FrontendBucket", {
      // websiteIndexDocument: 'index.html', // Remove website config for OAC
      publicReadAccess: false,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: cdk.RemovalPolicy.DESTROY, // Change to RETAIN for production
      autoDeleteObjects: true, // Change to false for production
    });

    // Origin Access Control (OAC)
    const cfnOriginAccessControl = new cloudfront.CfnOriginAccessControl(
      this,
      "OAC",
      {
        originAccessControlConfig: {
          name: "FrontendOAC",
          originAccessControlOriginType: "s3",
          signingBehavior: "always",
          signingProtocol: "sigv4",
        },
      },
    );

    // API Gateway Origin
    const apiOrigin = new origins.HttpOrigin(props.apiGatewayDomain, {
      protocolPolicy: cloudfront.OriginProtocolPolicy.HTTPS_ONLY,
      originPath: "/prod",
      customHeaders: {
        "X-Origin-Verify":
          process.env.API_GATEWAY_SECRET || "change-me-in-prod-secret",
      },
    });

    // CloudFront Function for SPA routing (resolve index.html for subdirectories)
    const rewriteFunction = new cloudfront.Function(this, "RewriteFunction", {
      code: cloudfront.FunctionCode.fromInline(`
function handler(event) {
    var request = event.request;
    var uri = request.uri;
    
    // Check whether the URI is missing a file name.
    if (uri.endsWith('/')) {
        request.uri += 'index.html';
    } 
    // Check whether the URI is missing a file extension.
    else if (!uri.includes('.')) {
        request.uri += '/index.html';
    }
    
    return request;
}
      `),
    });

    // Optional Custom Domain Configuration
    const customDomainName = process.env.CUSTOM_DOMAIN_NAME;
    const certificateArn = process.env.CERTIFICATE_ARN;

    let certificate: acm.ICertificate | undefined = undefined;
    let domainNames: string[] | undefined = undefined;

    if (customDomainName && certificateArn) {
      certificate = acm.Certificate.fromCertificateArn(
        this,
        "CustomDomainCert",
        certificateArn,
      );
      domainNames = [customDomainName];
    }

    // CloudFront Distribution
    this.distribution = new cloudfront.Distribution(
      this,
      "FrontendDistribution",
      {
        certificate: certificate,
        domainNames: domainNames,
        minimumProtocolVersion: certificate
          ? cloudfront.SecurityPolicyProtocol.TLS_V1_3_2025
          : cloudfront.SecurityPolicyProtocol.TLS_V1_2_2021,
        defaultBehavior: {
          origin: origins.S3BucketOrigin.withBucketDefaults(bucket),
          viewerProtocolPolicy:
            cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
          functionAssociations: [
            {
              function: rewriteFunction,
              eventType: cloudfront.FunctionEventType.VIEWER_REQUEST,
            },
          ],
        },
        additionalBehaviors: {
          "/api/*": {
            origin: apiOrigin,
            viewerProtocolPolicy:
              cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
            allowedMethods: cloudfront.AllowedMethods.ALLOW_ALL,
            cachePolicy: cloudfront.CachePolicy.CACHING_DISABLED,
            originRequestPolicy:
              cloudfront.OriginRequestPolicy.ALL_VIEWER_EXCEPT_HOST_HEADER,
          },
        },
        defaultRootObject: "index.html",
        errorResponses: [
          {
            httpStatus: 404, // S3 returns 404 if file not found (or 403 if permissive)
            responseHttpStatus: 200,
            responsePagePath: "/index.html",
          },
          {
            httpStatus: 403, // Handle S3 403 (access denied) as 200 index.html for SPA
            responseHttpStatus: 200,
            responsePagePath: "/index.html",
          },
        ],
      },
    );

    // Associate OAC with Distribution
    const cfnDistribution = this.distribution.node
      .defaultChild as cloudfront.CfnDistribution;
    cfnDistribution.addPropertyOverride(
      "DistributionConfig.Origins.0.OriginAccessControlId",
      cfnOriginAccessControl.attrId,
    );
    // Remove the OAI that S3Origin adds by default to avoid conflict
    cfnDistribution.addPropertyOverride(
      "DistributionConfig.Origins.0.S3OriginConfig.OriginAccessIdentity",
      "",
    );

    // Grant CloudFront access to S3 Bucket
    bucket.addToResourcePolicy(
      new cdk.aws_iam.PolicyStatement({
        actions: ["s3:GetObject"],
        resources: [bucket.arnForObjects("*")],
        principals: [
          new cdk.aws_iam.ServicePrincipal("cloudfront.amazonaws.com"),
        ],
        conditions: {
          StringEquals: {
            "AWS:SourceArn": `arn:aws:cloudfront::${cdk.Stack.of(this).account}:distribution/${this.distribution.distributionId}`,
          },
        },
      }),
    );

    // S3 Deployment moved to shell script for two-phase deployment

    // Outputs
    new cdk.CfnOutput(this, "FrontendUrl", {
      value: `https://${this.distribution.distributionDomainName}`,
      description: "Frontend URL",
    });

    new cdk.CfnOutput(this, "FrontendBucketName", {
      value: bucket.bucketName,
      description: "Frontend S3 Bucket Name",
    });
  }
}
