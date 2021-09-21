# Pleco
![Push](https://github.com/Qovery/pleco/workflows/Push/badge.svg)
[![DockerHub](https://img.shields.io/badge/DockerHub-pleco-blue?url=https://hub.docker.com/repository/docker/qoveryrd/pleco)](https://hub.docker.com/repository/docker/qoveryrd/pleco)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/pleco)](https://artifacthub.io/packages/search?repo=pleco)
[![Powered](https://img.shields.io/badge/Powered%20by-Qovery-blueviolet)](https://www.qovery.com)

<p align="center">
    <img src="./assets/pleco_logo.png" width=420 />
</p>

Automatically remove cloud and kubernetes resources based on a time to leave tag, **ttl**.

Protect resources from deletion with a protection tag, **do_no_delete**.

NOTE: this project is used in Qovery's production environment

---
## Supported resources
- [X] Kubernetes
  - [X] Namespaces
- [X] AWS 
  - [X] Document DB databases
  - [X] Document DB subnet groups
  - [X] Elasticache databases
  - [ ] Elasticache subnet groups
  - [X] RDS databases
  - [X] RDS subnet groups
  - [X] RDS parameter groups
  - [X] EBS volumes
  - [X] ELB load balancers
  - [X] EC2 Key pairs
  - [X] ECR repositories
  - [X] EKS clusters
  - [X] IAM groups
  - [X] IAM users
  - [X] IAM policies
  - [X] IAM roles
  - [X] Cloudwatch logs
  - [X] KMS keys
  - [X] VPC vpcs
  - [X] VPC internet gateways
  - [X] VPC nat gateways
  - [X] VPC Elastic IP
  - [X] VPC route tables
  - [X] VPC subnets
  - [X] VPC security groups
  - [X] S3 buckets 
- [ ] DIGITAL OCEAN
- [ ] AZURE
- [ ] GCP

---
## Installation

You can find a helm chart [here](https://artifacthub.io/packages/helm/pleco/pleco), a docker image [here](https://hub.docker.com/r/qoveryrd/pleco) and all binaries are on [github](https://github.com/Qovery/pleco/releases).

---
## Requirements

In order to make pleco check and clean expired aws resources you need to set the following environment variables:
```bash
$ export AWS_ACCESS_KEY_ID=<access_key>
$ export AWS_SECRET_ACCESS_KEY=<secret_key>
```
---
## Basic command

A pleco command has the following structure:
```bash
pleco start [options]
```

### General options
#### Debug Level
You can set the debug level with:
```bash
--level <log level>
```
Default is "info"

#### Check's interval
You can set the interval between two pleco's check with:
```bash
--check-interval, -i <time in seconds>
```
Default is "120"

#### Dry Run
If you disable dry run, pleco will delete expired resources. 
If not it will only tells you how many resources are expired.

You can disable dry-run with:
```bash
--disable-dry-run, -y
```
Default is "false"

### AWS options
#### Region selector
When pleco's look for expired resources, it will do it by aws region.

You can set region(s) with:
```bash
--aws-regions, -a <region(s)>
```

For exemple:
```bash
-a eu-west-3,us-east-2
```

#### Resources Selector
When pleco is running you have to specify which resources expiration will be checked.

Here are some the resources you can check:
```bash
--enable-eks, -e # Enable EKS watch
--enable-iam, -u # Enable IAM watch (groups, policies, roles, users)
```

#### Example
```bash
pleco start --level debug -i 240 -a eu-west-3 -e -r -m -c -l -b -p -s -w -n -u -z -o -y
```
