ecschedule
=======

[![Test Status](https://github.com/Songmu/ecschedule/workflows/test/badge.svg?branch=main)][actions]
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)][license]
[![GoDoc](https://godoc.org/github.com/Songmu/ecschedule?status.svg)][godoc]

[actions]: https://github.com/Songmu/ecschedule/actions?workflow=test
[license]: https://github.com/Songmu/ecschedule/blob/main/LICENSE
[godoc]: https://godoc.org/github.com/Songmu/ecschedule

ecschedule is a tool to manage ECS Scheduled Tasks.

## Synopsis

```command
% ecschedule [dump|apply|run|diff] -conf ecschedule.yaml -rule $ruleName
```

## Description

The ecschedule manages ECS Schedule tasks using a YAML configuration file like following.

```yaml
region: us-east-1
cluster: clusterName
rules:
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
        with:
          version: v0.3.1
      - run: |
          ecschedule -conf ecschedule.yaml apply -all
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

## Functions

You can use following functions in the configuration file.

- `env`
    - expand environment variable or using default value
    - `{{ env "ENV_NAME" "DEFAULT_VALUE" }}`
- `must_env`
    - expand environment variable
    - `{{ must_env "ENV_NAME" }}`

inspired by [ecspresso](https://github.com/kayac/ecspresso).

# Plugins

## tfstate

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

## Author

[Songmu](https://github.com/Songmu)
