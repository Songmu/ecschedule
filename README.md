ecschedule
=======

[![Test Status](https://github.com/Songmu/ecschedule/workflows/test/badge.svg?branch=main)][actions]
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)][license]
[![PkgGoDev](https://pkg.go.dev/badge/github.com/Songmu/ecschedule)][PkgGoDev]

[actions]: https://github.com/Songmu/ecschedule/actions?workflow=test
[license]: https://github.com/Songmu/ecschedule/blob/main/LICENSE
[PkgGoDev]: https://pkg.go.dev/github.com/Songmu/ecschedule

ecschedule is a tool to manage ECS Scheduled Tasks.

## Synopsis

```command
% ecschedule [dump|apply|run|diff] -conf ecschedule.yaml -rule $ruleName
```

## Description

The ecschedule manages ECS Schedule tasks using a configuration file (YAML, JSON or Jsonnet format) like following.

```yaml
region: us-east-1
cluster: clusterName
rules:
trackingId: trackingId1
- name: taskName1
  description: task 1
  scheduleExpression: cron(30 15 ? * * *)
  taskDefinition: taskDefName
  containerOverrides:
  - name: containerName
    command: [subcommand1, arg]
    environment:
      HOGE: foo
      FUGA: {{ must_env `APP_FUGA` }}
- name: taskName2
  description: task2
  scheduleExpression: cron(30 16 ? * * *)
  taskDefinition: taskDefName2
  containerOverrides:
  - name: containerName2
    command: [subcommand2, arg]
```

## Installation

```console
% brew install Songmu/tap/ecschedule
# or
% go install github.com/Songmu/ecschedule/cmd/ecschedule@latest
```

### GitHub Actions

Action Songmu/ecschedule@main installs ecschedule binary for Linux into /usr/local/bin. This action runs install only.

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: Songmu/ecschedule@main
      - run: |
          ecschedule -conf ecschedule.yaml apply -all
```

### aqua

A declarative CLI Version Manager [aqua](https://aquaproj.github.io/) can install ecschedule.

```console
% aqua g -i Songmu/ecschedule
```

## Quick Start

### dump configuration YAML

```console
% ecschedule dump --cluster clusterName --region us-east-1 > ecschedule.yaml
```

edit and adjust configuration file after it.

### apply new or updated rule

```console
% ecschedule -conf ecschedule.yaml apply -rule $ruleName
```

Before you apply it, you can check the diff in the following way.

```console
% ecschedule -conf ecschedule.yaml diff -rule $ruleName
```

### run rule

Execute `run` subcommand when want execute arbitrary timing.

```console
% ecschedule -conf ecschedule.yaml run -rule $ruleName
```

### Using the `-prune` option to manage rules

In version `v0.9.1` and earlier, when rules were renamed or deleted from the configuration, the old rules remained and had to be deleted manually. With the `-prune` option introduced in `v0.10.0`, you can now automatically remove these old rules.

```console
% ecschedule -conf ecschedule.yaml apply -all -prune
```

To see which rules would be deleted without actually removing them, you can use either `diff` or `apply` with `-dry-run` option:

```console
% ecschedule -conf ecschedule.yaml diff -all -prune
% ecschedule -conf ecschedule.yaml apply -all -prune -dry-run
```

A `trackingId` is an optional key in a configuration file. If the `trackingId` is not explicitly specified, the cluster name will be used as the `trackingId` by default.
When you explicitly specify the `trackingId`, it enables you to detect rule deletions for each file when executing multiple configuration files at different times.

### Validation

The `apply` and `run` commands **always perform validation** (env, tfstate, ssm, task definition) before execution and cannot be disabled.

The `diff` command **does not perform validation by default** for quick review. To enable validation (recommended for CI/CD pipelines), use the `-validate` flag:

```console
% ecschedule -conf ecschedule.yaml diff -all -prune -validate -u
```

This performs the same validation as `apply` and `run`, with a slight overhead, but ensures complete verification before applying changes.

### Parallel Execution

The `diff` command supports parallel execution for improved performance with many rules:

```console
% ecschedule -conf ecschedule.yaml diff -all -parallelism 10
```

- Default: `parallelism=1` (sequential, backward compatible)
- Recommended: 1-10 (due to AWS API rate limits)
- Note: Output order is not guaranteed when parallelism > 1

## Log Format

### Unified diff format

ecschedule supports unified diff format (similar to `git diff`) with `-u` flag for both `diff` and `apply` commands:

```console
% ecschedule -conf ecschedule.yaml diff -all -u
% ecschedule -conf ecschedule.yaml apply -all -dry-run -u
```

The `diff` command with `-u` flag outputs pure diff content without log prefixes or headers, making it suitable for piping to other tools.
The `apply` command includes progress logs even with `-u` flag.

### Color control

Colored output can be disabled with `--no-color` flag or environment variables:

```console
% ecschedule -conf ecschedule.yaml diff -all -u --no-color
% NO_COLOR=1 ecschedule -conf ecschedule.yaml diff -all -u
% ECSCHEDULE_COLOR=false ecschedule -conf ecschedule.yaml apply -all -dry-run -u
```

Environment variables:
- `NO_COLOR`: If set (value doesn't matter), disable colored output. Follows the [NO_COLOR standard](https://no-color.org/).
- `ECSCHEDULE_COLOR`: Set to `false` or `0` to disable colored output (unified diff format only). Default is `true`.

Priority: `--no-color` flag > `NO_COLOR` > `ECSCHEDULE_COLOR` > default (colored).

**Note**: The `--no-color` flag is a subcommand-level flag and only affects unified diff format. The default pretty format always displays with colors.

## Functions

You can use following functions in the configuration file.

- `env`
    - expand environment variable or using default value
    - `{{ env "ENV_NAME" "DEFAULT_VALUE" }}`
- `must_env`
    - expand environment variable
    - `{{ must_env "ENV_NAME" }}`

inspired by [ecspresso](https://github.com/kayac/ecspresso).

## Plugins

### tfstate

tfstate plugin introduces a template function `tfstate`.

```yaml
region: us-east-1
cluster: api
role: ecsEventsRole
rules:
- name: hoge-task-name
  description: hoge description
  scheduleExpression: cron(0 0 * * ? *)
  taskDefinition: task1
  group: xxx
  platform_version: 1.4.0
  launch_type: FARGATE
  network_configuration:
    aws_vpc_configuration:
      subnets:
      - {{ tfstate `aws_subnet.private-a.id` }}
      - {{ tfstate `aws_subnet.private-c.id` }}
      security_groups:
      - {{ tfstatef `data.aws_security_group.default['%s'].id` `first` }}
      - {{ tfstatef `data.aws_security_group.default['%s'].id` `second` }}
      assign_public_ip: ENABLED
  containerOverrides:
  - name: container1
    command: ["subcmd", "argument"]
    environment:
      HOGE_ENV: {{ env "DUMMY_HOGE_ENV" "HOGEGE" }}
    cpu: 1024
    memory: 1024
    memoryReservation: 512
  dead_letter_config:
    sqs: queue1
  propagateTags: TASK_DEFINITION
plugins:
- name: tfstate
  config:
    path: testdata/terraform.tfstate    # path to tfstate file
      # or url: s3://my-bucket/terraform.tfstate
```

`{{ tfstate "resource_type.resource_name.attr" }}` will expand to an attribute value of the resource in tfstate.

`{{ tfstatef "resource_type.resource_name['%s'].attr" "index" }}` is similar to `{{ tfstatef "resource_type.resource_name['index'].attr" }}`.
This function is useful to build a resource address with environment variables.

```
{{ tfstatef `aws_subnet.ecs['%s'].id` (must_env `SERVICE`) }}
```

#### Multiple tfstate support

You can specify multiple tfstate files. Specify the `func_prefix` option to avoid conflicts between functions.

```yaml
plugins:
- name: tfstate
  func_prefix: first_
  config:
    path: testdata/first_terraform.tfstate    # path to tfstate file
- name: tfstate
  func_prefix: second_
  config:
    path: testdata/second_terraform.tfstate    # path to tfstate file
```

In this case, the function must be called by the `plugin` function.

The `plugin` function takes the prefixed function name as the first argument and the function arguments as the second or later arguments.

```
{{ plugin `first_tfstate` `aws_subnet.private-a.id` }}
{{ plugin `second_tfstate` `aws_subnet.private-a.id` }}
```

### ssm

ssm plugin introduces a template function `ssm`.

```yaml
region: us-east-1
cluster: api
role: ecsEventsRole
rules:
- name: fuga-task-name
  description: fuga description
  scheduleExpression: cron(0 0 * * ? *)
  taskDefinition: task1
  group: xxx
  platform_version: 1.4.0
  launch_type: FARGATE
  network_configuration:
    aws_vpc_configuration:
      subnets:
      - {{ ssm `/subnet/private/a/id` }} # String type.
      - {{ ssm `/subnet/private/c/id` }}
      security_groups:
      - {{ ssm `security_group/default/id` 1 }} # StringList type.
      - {{ ssm `security_group/default/id` 2 }}
      assign_public_ip: ENABLED
  containerOverrides:
  - name: container1
    command: ["subcmd", "argument"]
    environment:
      FUGA_ENV: {{ ssm "/path/to/secretstring/fuga" }} # SecureString type.
    cpu: 1024
    memory: 1024
    memoryReservation: 512
  dead_letter_config:
    sqs: queue1
  propagateTags: TASK_DEFINITION
plugins:
- name: ssm
```

`{{ ssm "/path/to/parameter" }}` will retrieve a parameter from the AWS Systems Manager Parameter Store.

This function supports String, StringList, and SecureString types.

## Pitfalls

### Rule Name Uniqueness and Overwrite Risks

ecschedule is designed to guarantee the uniqueness of job definitions by rule name in the configuration file.

If ecschedule is run in an environment where a Rule that is not managed by ecschedule already exists, ecschedule will overwrite that Rule. If you do not intend to overwrite, please ensure that the names written in the configuration file do not duplicate with existing Rules.

### Note on Previous Versions

In versions `v0.9.1` and earlier, there were issues related to rule name changes causing garbage definitions and rules not being deleted from AWS when removed from the configuration file. These issues have been addressed in version `v0.10.0` with the introduction of the `-prune` option.

## Author

[Songmu](https://github.com/Songmu)
