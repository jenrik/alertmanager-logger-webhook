# Alertmanager-Logger-Webhook

Logs alerts from [AlertManager](https://github.com/prometheus/alertmanager) to log files for easy and reliable collection.

alertmanager-logger-webhook is designed to minimize the chance of losing events by implementing graceful shutdown and managed log file rotation.

## Setup
The logger by itself does not need to be configured in most cases, but you will need to configure you AlertManager instance to send alerts to the logger.
The following is an example of how you can configure you AlertManager:

```yaml
route:
  receiver: logger
  continue: true
  # Do not bundle, just send immediately
  group_wait: 0s
  group_interval: 0s
  routes:
    - # Your routes here

receivers:
  - name: logger
    webhooks_configs:
      - url: "http://alertmanager-logger-webhook:8080/log"
        send_resolved: true
        # Disable bundling for easier data processing later
        max_alerts: 1
```

_Remember to store the produced log files somewhere safe and useful._