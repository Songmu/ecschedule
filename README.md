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
- name: taskName1
  description: task 1
  scheduledExpression: cron(30 15 ? * * *)
  taskDefinition: taskDefName
  containerOverrides:
  - name: containerName
    command: [subcommand1, arg]
    environment:
      HOGE: foo
      FUGA: {{ must_env `APP_FUGA` }}
- name: taskName2
  description: task2
  scheduledExpression: cron(30 16 ? * * *)
  taskDefinition: taskDefName2
  containerOverrides:
  - name: containerName2
    command: [subcommand2, arg]
```

## Installation

```console
% go get github.com/Songmu/ecschedule/cmd/ecschedule
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

## Functions

You can use following functions in the configuration file.

- `env`
    - expand environment variable or using default value
    - `{{ env "ENV_NAME" "DEFAULT_VALUE" }}`
- `must_env`
    - expand environment variable
    - `{{ must_env "ENV_NAME" }}`

inspired by [ecspresso](https://github.com/kayac/ecspresso).

## Author

[Songmu](https://github.com/Songmu)
