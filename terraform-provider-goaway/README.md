# Terraform Provider for GoAway

Manage GoAway DNS sinkhole resources declaratively with Terraform (`goaway_upstream`, `goaway_resolution`, `goaway_whitelist_domain`, `goaway_group`).

## Build & Test

```bash
cd terraform-provider-goaway
go test -race ./...
go build -o terraform-provider-goaway .
```

See [examples/main.tf](./examples/main.tf) for usage.
