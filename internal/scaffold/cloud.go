package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

type iacFiles struct {
	files map[string]string
}

var cloudIaC = map[string]map[string]iacFiles{
	"aws": {
		"terraform": {map[string]string{
			"infra/terraform/main.tf":      awsTFMain,
			"infra/terraform/variables.tf": awsTFVars,
		}},
		"cdk": {map[string]string{
			"infra/cdk/lib/stack.ts": awsCDKStack,
			"infra/cdk/bin/app.ts":   awsCDKApp,
		}},
		"sam": {map[string]string{
			"infra/sam/template.yaml": awsSAMTemplate,
		}},
		"pulumi": {map[string]string{
			"infra/pulumi/index.ts": pulumiAWS,
		}},
	},
	"azure": {
		"terraform": {map[string]string{
			"infra/terraform/main.tf":      azureTFMain,
			"infra/terraform/variables.tf": azureTFVars,
		}},
		"bicep": {map[string]string{
			"infra/bicep/main.bicep": azureBicep,
		}},
		"pulumi": {map[string]string{
			"infra/pulumi/index.ts": pulumiAzure,
		}},
	},
	"gcp": {
		"terraform": {map[string]string{
			"infra/terraform/main.tf":      gcpTFMain,
			"infra/terraform/variables.tf": gcpTFVars,
		}},
		"pulumi": {map[string]string{
			"infra/pulumi/index.ts": pulumiGCP,
		}},
		"deployment-manager": {map[string]string{
			"infra/deployment-manager/config.yaml": gcpDM,
		}},
	},
}

func writeCloudIaC(root, cloud, iac string) error {
	cloudMap, ok := cloudIaC[cloud]
	if !ok {
		return fmt.Errorf("unknown cloud: %s", cloud)
	}
	tmpl, ok := cloudMap[iac]
	if !ok {
		return fmt.Errorf("unknown IaC %q for cloud %q", iac, cloud)
	}
	for rel, content := range tmpl.files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return nil
}

// ── AWS ──────────────────────────────────────────────────────────────────────

const awsTFMain = `terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

provider "aws" {
  region = var.region
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"
  name    = var.project_name
  cidr    = "10.0.0.0/16"
  azs             = ["${var.region}a", "${var.region}b"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]
  enable_nat_gateway = true
}
`

const awsTFVars = `variable "project_name" {
  type = string
}
variable "region" {
  type    = string
  default = "us-east-1"
}
`

const awsCDKStack = `import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import { Construct } from 'constructs';

export class AppStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);
    new ec2.Vpc(this, 'Vpc', { maxAzs: 2 });
  }
}
`

const awsCDKApp = `#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { AppStack } from '../lib/stack';

const app = new cdk.App();
new AppStack(app, 'AppStack');
`

const awsSAMTemplate = `AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31

Globals:
  Function:
    Runtime: python3.12
    Timeout: 30

Resources:
  ApiFunction:
    Type: AWS::Serverless::Function
    Properties:
      Handler: src.adapters.api.handler.lambda_handler
      Events:
        Api:
          Type: HttpApi
`

const pulumiAWS = `import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const vpc = new aws.ec2.Vpc("app-vpc", {
  cidrBlock: "10.0.0.0/16",
  tags: { Name: pulumi.getProject() },
});

export const vpcId = vpc.id;
`

// ── Azure ─────────────────────────────────────────────────────────────────────

const azureTFMain = `terraform {
  required_version = ">= 1.9"
  required_providers {
    azurerm = { source = "hashicorp/azurerm", version = "~> 3.0" }
  }
}

provider "azurerm" {
  features {}
}

resource "azurerm_resource_group" "rg" {
  name     = var.project_name
  location = var.location
}
`

const azureTFVars = `variable "project_name" { type = string }
variable "location" { type = string; default = "East US" }
`

const azureBicep = `param projectName string
param location string = resourceGroup().location
`

const pulumiAzure = `import * as azure from "@pulumi/azure-native";

const rg = new azure.resources.ResourceGroup("app-rg");
export const rgName = rg.name;
`

// ── GCP ───────────────────────────────────────────────────────────────────────

const gcpTFMain = `terraform {
  required_version = ">= 1.9"
  required_providers {
    google = { source = "hashicorp/google", version = "~> 5.0" }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
`

const gcpTFVars = `variable "project_id" { type = string }
variable "region" { type = string; default = "us-central1" }
`

const pulumiGCP = `import * as gcp from "@pulumi/gcp";

const network = new gcp.compute.Network("app-network");
export const networkId = network.id;
`

const gcpDM = `imports: []
resources:
  - name: app-network
    type: compute.v1.network
    properties:
      autoCreateSubnetworks: false
`
