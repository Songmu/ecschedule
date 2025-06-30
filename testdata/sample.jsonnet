local envs = import 'envs.libsonnet';

{
  "region": "us-east-1",
  "cluster": "api",
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
          "base": 1,
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
