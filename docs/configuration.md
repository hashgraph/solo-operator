# Configuration

The operator (`cmd/main.go`) accepts the following command-line flags:

| Flag | Description | Default |
|------|-------------|---------|
| `--version` | Print version information and exit | `false` |
| `--log-level` | Log level: trace, debug, info, warn, error | `info` |
| `--console-logging` | Use the human-readable console writer instead of structured JSON | `false` |
| `--leader-elect` | Enable leader election for HA | `false` |
| `--metrics-bind-address` | Metrics endpoint address (`:8443` HTTPS, `:8080` HTTP, `0` to disable) | `0` |
| `--metrics-secure` | Serve the metrics endpoint over HTTPS | `true` |
| `--enable-http2` | Enable HTTP/2 for the metrics server | `false` |
| `--health-probe-bind-address` | Health probe endpoint | `:8081` |
| `--registry-order` | Order registries are checked during upgrade Prepare: `Random` or `Sequential` | `Random` |
| `--node-active-timeout` | Backstop deadline for a node to reach platform status ACTIVE after an upgrade | `10m` |
| `--node-active-alert-interval` | Interval between repeated NodeActiveDeadlineExceeded warnings | `5m` |
