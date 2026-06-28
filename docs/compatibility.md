# Compatibility Validation

Helios keeps a stable semantic runtime layer while built-in adapters track fast
moving agent CLIs. Use the compatibility harness before enabling an adapter in a
host application environment.

## CLI Probe

```bash
go run ./cmd/helios-compat -agent hermes -cli hermes
go run ./cmd/helios-compat -agent open_code -cli opencode
go run ./cmd/helios-compat -agent claude_code -cli claude-agent-acp
go run ./cmd/helios-compat -agent open_claw -cli openclaw
```

The command exits with a non-zero status if any selected scenario fails.

## Scenario Matrix

| Scenario | Meaning | Required for |
| --- | --- | --- |
| `detect` | Starts the CLI and reads runtime capabilities. | Adapter setup pages and health checks. |
| `one_shot` | Runs a short prompt through `Engine.Run`. | Colink-style one-time jobs. |
| `resident` | Starts a session and prompts it. | Callme-style resident conversations. |
| `resume` | Starts with `ResumeSessionID`. | Long-running sessions and process restarts. |
| `elicitation` | Prompts an agent path expected to ask a question. | Human-in-the-loop flows. |
| `capabilities` | Reports normalized capability fields. | Feature flags and UI affordances. |

## Built-In Adapter Expectations

| Adapter | SDK coverage | Real CLI check |
| --- | --- | --- |
| `hermes` | Config rendering, ACP session flow, event normalization. | Verify installed `hermes acp` protocol behavior. |
| `open_code` | Config injection, pure mode, ACP question tool setup. | Verify installed `opencode` ACP bridge and model config. |
| `claude_code` | `claude-agent-acp` environment wiring. | Verify installed bridge version and auth environment. |
| `open_claw` | ACP bridge argument construction. | Verify gateway URL, token, and bridge lifecycle. |

## Recommended Release Gate

Before tagging a Helios release:

1. Run `go test ./...`.
2. Run `go run ./cmd/helios-compat` against every installed built-in adapter.
3. Record CLI versions and probe output in release notes.
4. Keep raw compatibility output when investigating protocol drift.
