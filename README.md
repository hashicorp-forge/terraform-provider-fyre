# terraform-provider-fyre

A Terraform provider for managing virtual machines, clusters, and stencils in
the IBM Fyre development cloud environment. This is for internal user only.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.26

## Building the Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Using the Provider

Fill this in for each provider

## Developing the Provider

To generate or update documentation and/or fyre API client, run `make generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```
