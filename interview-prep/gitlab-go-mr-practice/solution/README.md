# endpoint-monitor

A small service that periodically checks a list of HTTP endpoints and exposes
their latest health status over HTTP.

## Usage

```shell
go run ./cmd/monitor -config endpoints.example.json -interval 30s
curl localhost:8080/status
curl localhost:8080/stats
```

To check every endpoint once (for example from a CI job) and exit with a
non-zero status if any of them is unhealthy:

```shell
go run ./cmd/monitor -config endpoints.example.json -once
```

### Flags

| Flag        | Default        | Description                              |
|-------------|----------------|------------------------------------------|
| `-config`   | `endpoints.json` | Path to the JSON list of endpoints     |
| `-interval` | `30s`          | How often all endpoints are checked      |
| `-timeout`  | `5s`           | Per-request timeout                      |
| `-addr`     | `:8080`        | Address the status server listens on     |
| `-concurrency` | `10`        | Maximum number of endpoints checked in parallel |
| `-once`     | `false`        | Check all endpoints once and exit        |

## Development

```shell
go test ./...
```
