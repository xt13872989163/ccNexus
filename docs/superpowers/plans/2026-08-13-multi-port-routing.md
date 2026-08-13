# Multi-Port Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow ccNexus to listen on multiple ports and assign a default endpoint per port while preserving Claude Code and Codex request compatibility.

**Architecture:** Add persisted `portBindings` to configuration. Each binding creates an HTTP listener using the same proxy handlers and injects a fallback endpoint into request context; explicit endpoint header, model prefix, and query parameter selection remains authoritative. The existing `port` remains the primary listener and backward-compatible configuration field.

**Tech Stack:** Go, net/http, existing SQLite config adapter, Go tests.

---

### Task 1: Add port binding configuration and persistence

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/multi_port_test.go`

- [ ] Add `PortBinding` with `Port` and `Endpoint` JSON fields, `PortBindings []PortBinding` on `Config`, defaults, validation for range/duplicate ports and endpoint names, and JSON storage under `portBindings`.
- [ ] Add tests covering round-trip load/save and validation failures.

### Task 2: Route requests by listener port

**Files:**
- Modify: `internal/proxy/endpoint_resolver.go`
- Modify: `internal/proxy/proxy_request.go`
- Modify: `internal/proxy/proxy.go`
- Test: `internal/proxy/multi_port_test.go`

- [ ] Add request-context helpers for a listener default endpoint.
- [ ] Resolve explicit header/model/query selectors first, then listener default endpoint.
- [ ] Start one HTTP server per configured binding, with the primary `port` listener retained for compatibility and graceful shutdown of all listeners.
- [ ] Add tests for resolver precedence and listener route construction.

### Task 3: Expose configuration API and documentation

**Files:**
- Modify: `cmd/server/webui/api/config.go`
- Modify: `docs/configuration.md`
- Modify: `README.md`

- [ ] Include `portBindings` in config GET/PUT and validate endpoint references and port ranges.
- [ ] Document Claude Code and Codex base URLs for separate ports and the selector precedence.

### Task 4: Verify

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./...`.
- [ ] Review `git diff` and report branch name and verification results.
