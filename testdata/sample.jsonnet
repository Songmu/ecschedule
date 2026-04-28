local envs = import 'envs.libsonnet';

{
  "region": std.extVar('REGION'),
  "cluster": std.extVar('CLUSTER'),
  "role": "ecsEventsRole",
  "rules": [
    {
      "name": "hoge-task-name",
      "description": "hoge description",
      "scheduleExpression": "cron(0 0 * * ? *)",
      "taskDefinition": "task1",
      "group": "xxx",
      "platform_version": "1.4.0",
      "launch_type": "FARGATE",
      "capacityProviderStrategy": [
        {
          "capacityProvider": "FARGATE",
          "base": std.extVar('BASE'),
          "weight": 1
        }
      ],
      "network_configuration": envs.network_configuration,
      "taskOverride": {
        "cpu": "4096",
        "memory": "16384"
      },
      "containerOverrides": [
        {
          "name": "container1",
          "command": [
            "subcmd",
            "argument"
          ],
          "environment": {
            "HOGE_ENV": "{{ env `DUMMY_HOGE_ENV` `HOGEGE` }}"
          }
        }
      ],
      "dead_letter_config": {
        "sqs": "queue1"
      },
      "propagateTags": "TASK_DEFINITION"
    }
  ]
}
