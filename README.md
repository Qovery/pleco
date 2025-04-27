# Pleco

![Push](https://github.com/Qovery/pleco/workflows/Push/badge.svg)
[![DockerHub](https://img.shields.io/badge/DockerHub-pleco-blue?url=https://hub.docker.com/repository/docker/qoveryrd/pleco)](https://hub.docker.com/repository/docker/qoveryrd/pleco)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/pleco)](https://artifacthub.io/packages/search?repo=pleco)
[![Powered](https://img.shields.io/badge/Powered%20by-Qovery-blueviolet)](https://www.qovery.com)

<p align="center">
    <img src="./assets/pleco_logo.png" width=420 />
</p>

Automatically remove cloud and kubernetes resources based on a time to leave tag, **ttl**.

Protect resources from deletion with a protection tag, **do_not_delete**.

NOTE: this project is used in Qovery's production environment

---

Check out our Blog announcement of Pleco: https://www.qovery.com/blog/announcement-of-pleco-the-open-source-kubernetes-and-cloud-services-garbage-collector

---

## Supported resources

- [x] Kubernetes
  - [x] Namespaces
- [x] AWS
  - [x] Document DB databases
  - [x] Document DB subnet groups
  - [x] Elasticache databases
  - [x] Elasticache subnet groups
  - [x] RDS databases
  - [x] RDS subnet groups
  - [x] RDS parameter groups
  - [x] RDS snapshots
  - [x] EBS volumes
  - [x] ELB load balancers
  - [x] EC2 Key pairs
  - [x] ECR repositories
  - [x] EKS clusters
  - [x] IAM groups
  - [x] IAM users
  - [x] IAM policies
  - [x] IAM roles
  - [x] IAM OpenId Connect provider
  - [x] Cloudwatch logs
  - [x] KMS keys
  - [x] VPC vpcs
  - [x] VPC internet gateways
  - [x] VPC nat gateways
  - [x] VPC Elastic IP
  - [x] VPC route tables
  - [x] VPC subnets
  - [x] VPC security groups
  - [x] S3 buckets
  - [x] Lambda Functions
  - [x] SQS Queues
  - [x] Step Functions
  - [x] EC2 instances
- [x] SCALEWAY
  - [x] Kubernetes clusters
  - [x] Database instances
  - [x] Load balancers
  - [x] Detached volumes
  - [x] S3 Buckets
  - [x] Unused Security Groups
  - [x] Orphan IPs
  - [x] VPCs
  - [x] Private networks
- [x] DIGITAL OCEAN
  - [x] Kubernetes clusters
  - [x] Database instances
  - [x] Load balancers
  - [x] Detached volumes
  - [x] S3 Buckets
  - [x] Droplet firewalls
  - [x] Unused VPCs
- [x] GCP
  - [x] Cloud Storage Buckets
  - [x] Artifact Registry Repositories
  - [x] Kubernetes clusters
  - [x] Cloud run jobs
  - [x] Networks // via JSON tags in resource description because resource has no support for tags
  - [x] Routers // via JSON tags in resource description because resource has no support for tags
  - [x] Service accounts // via JSON tags in resource description because resource has no support for tags
- [x] AZURE
  - [x] Resource Group

---

## Installation

You can find a helm chart [here](https://artifacthub.io/packages/helm/pleco/pleco), a docker image [here](https://hub.docker.com/r/qoveryrd/pleco) and all binaries are on [github](https://github.com/Qovery/pleco/releases).

---

## Requirements

In order to make pleco check and clean expired resources you need to set the following environment variables:

#### For AWS

```bash
$ export AWS_ACCESS_KEY_ID=<access_key>
$ export AWS_SECRET_ACCESS_KEY=<secret_key>
```

#### For Scaleway

```bash
$ export SCW_ACCESS_KEY=<access_key>
$ export SCW_SECRET_KEY=<secret_key>
$ export SCW_VOLUME_TIMEOUT=<delay_before_detached_volume_deletion_in_hours_since_last_update> # default is 2 hours
```

#### For Digital Ocean

```bash
$ export DO_API_TOKEN=<your_do_api_token>
$ export DO_SPACES_KEY=<your_do_api_key_for_spaces>
$ export DO_SPACES_SECRET=<your_do_api_secret_for_spaces>
$ export DO_VOLUME_TIMEOUT=<delay_before_detached_volume_deletion_in_hours_since_creation> # default is 2 hours
```

#### For GCP

```bash
$ export GOOGLE_APPLICATION_CREDENTIALS=<path_to_your_credentials_json_file>
```

#### For Azure

```bash
$ export AZURE_CLIENT_ID=<client_id>
$ export AZURE_CLIENT_SECRET=<client_secret>
$ export AZURE_TENANT_ID=<tenant_id>
$ export AZURE_SUBSCRIPTION_ID=<subscription_id>
```

---

## Basic command

A pleco command has the following structure:

```bash
pleco start <cloud_provider> [options]
```

### General options

#### Connection Mode

You can set the connection mode with:

```bash
--kube-conn, -k <connection mode>
```

Default is "in"

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

For example:

```bash
-a eu-west-3,us-east-2
```

#### Resources Selector

When pleco is running you have to specify which resources expiration will be checked.

Here are some of the resources you can check:

```bash
--enable-eks, -e # Enable EKS watch
--enable-iam, -u # Enable IAM watch (groups, policies, roles, users)
```

#### Example

```bash
pleco start aws --level debug -i 240 -a eu-west-3 -e -r -m -c -l -b -p -s -w -n -u -z -o -f -x -q -y
```

### Scaleway options

#### Region selector

When pleco's look for expired resources, it will do it by [scaleway zone](https://registry.terraform.io/providers/scaleway/scaleway/latest/docs/guides/regions_and_zones).

You can set zone(s) with:

```bash
--scw-zones, -a <zone(s)>
```

For example:

```bash
-a fr-par-1
```

#### Resources Selector

When pleco is running you have to specify which resources expiration will be checked.

Here are some of the resources you can check:

```bash
--enable-cluster, -e # Enable cluster watch
```

#### Example

```bash
pleco start scaleway --level debug -i 240 -a fr-par-1 -e -r -o -l -b -s -p -y
```

### Digital Ocean options

#### Region selector

When pleco's look for expired resources, it will do it by [digital ocean region](https://docs.digitalocean.com/products/platform/availability-matrix/).

You can set zone(s) with:

```bash
--do-regions, -a <region(s)>
```

For example:

```bash
-a nyc3
```

#### Resources Selector

When pleco is running you have to specify which resources expiration will be checked.

Here are some of the resources you can check:

```bash
--enable-cluster, -e # Enable cluster watch
```

#### Example

```bash
pleco start do --level debug -i 240 -a nyc3 -e -r -s -l -b -f -v -y
```

### GCP options

#### Region selector

When pleco's look for expired resources, it will do it by [gcp_regions](https://cloud.google.com/compute/docs/regions-zones?hl=en).

You can set zone(s) with:

```bash
--gcp-regions, -a <region(s)>
```

For example:

```bash
-a europe-west9
```

#### Resources Selector

When pleco is running you have to specify which resources expiration will be checked.

Here are some of the resources you can check:

```bash
--enable-cluster # Enable cluster watch
--enable-object-storage # Enable object storage watch
--enable-artifact-registry # Enable artifact registry watch
--enable-network # Enable network watch
--enable-router # Enable router watch
--enable-iam # Enable IAM watch (service accounts)
```

#### Example

```bash
pleco start
  gcp
  --level
  debug
  -i
  240
  --enable-object-storage
  --enable-artifact-registry
  --enable-cluster
  --enable-network
  --enable-router
  --enable-iam
  --enable-job
  --gcp-regions
  europe-west9
  --disable-dry-run
```
