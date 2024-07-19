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

## Verifying artifacts

### Binary artifacts

```sh
VERSION=1.0.0
OS=linux
ARCH=amd64

curl -sSfL -o "binary-${OS}-${ARCH}" "https://github.com/jenrik/alertmanager-logger-webhook/releases/download/v${VERSION}/binary-${OS}-${ARCH}"
curl -sSfL -o "binary-${OS}-${ARCH}.intoto.jsonl" "https://github.com/jenrik/alertmanager-logger-webhook/releases/download/v${VERSION}/binary-${OS}-${ARCH}.intoto.jsonl"
slsa-verifier verify-artifact --print-provenance --source-uri=github.com/jenrik/alertmanager-logger-webhook --provenance-path binary-${OS}-${ARCH}.intoto.jsonl binary-${OS}-${ARCH}
```

# Container

```sh
VERSION=1.0.0
DIGEST=sha256:7f825a2d0bc99179a233fbabb8e01ecbda9c3e56f8feebbd7fa4cd4a9217c7cc
slsa-verifier verify-image --source-uri=github.com/jenrik/alertmanager-logger-webhook --source-tag=v${VERSION} ghcr.io/jenrik/alertmanager-logger-webhook:latest@${DIGEST}
```
