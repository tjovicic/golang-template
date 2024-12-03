# Golang template

A template for an HTTP API in Golang with some reasonable defaults:
- http timeouts
- graceful shutdown
- open telemetry setup
- cpu profiling

## API

### How to run

```bash
docker compose up api
```

### CPU Profiling
go tool pprof -seconds 30 -http=:8081 http://localhost:8080/debug/pprof/profile

### Memory Profiling
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/heap