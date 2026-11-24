# GoAway Helm Chart

Reproducible Kubernetes installation for [GoAway](https://github.com/rasty94/goaway), including declarative GitOps `settings.yaml` mounting (`api.readOnly: true`), non-root execution with `NET_BIND_SERVICE`, DNS UDP/TCP/DoQ services, and persistent SQLite storage.

## Install

```bash
helm upgrade --install goaway ./charts/goaway \
  --namespace dns --create-namespace
```

## Validate

```bash
helm lint ./charts/goaway
helm template goaway ./charts/goaway
```
