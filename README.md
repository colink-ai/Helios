# Helios

Helios is an embeddable AI application runtime SDK for building agentic products.

It provides runtime contracts, normalized event streams, adapter interfaces, and
session orchestration primitives. It intentionally does not own application
storage, tenancy, identity, billing, or product-specific data models.

## Design Principles

- Keep the SDK database-free. Applications persist sessions, messages, runs, and
  artifacts through their own stores.
- Normalize runtime events before they reach applications, so product code does
  not need to parse raw CLI or protocol output.
- Support embedded runtimes first, while leaving room for split worker
  deployments.
- Keep adapter interfaces stable enough for CLI agents, model runtimes, MCP
  tools, and future remote workers.

## Package Layout

- `contracts`: stable protocol and event types shared by hosts and adapters.
- `runtime`: core SDK abstractions, registry, session store interfaces, and
  runtime path helpers.
- `adapters/acp`: Agent Client Protocol transport and base adapter used by
  ACP-compatible agents.
- `adapters/hermes`: Hermes ACP adapter.
- `adapters/open_code`: OpenCode ACP adapter.
- `adapters/claude_code`: Claude Code adapter using `claude-agent-acp`.
- `adapters/open_claw`: OpenClaw ACP bridge adapter.
- `adapters/all`: helper that registers all built-in adapters.

## Persistence Boundary

Helios does not write to SQLite, MySQL, PostgreSQL, or any application database.
Host applications implement `runtime.EventSink` and `runtime.SessionStore` when
they want to persist runtime events or resume metadata.

## Runtime Modes

Helios supports both common product integration modes:

- One-shot runs: `runtime.Engine.Run` starts a temporary session, prompts the
  agent, streams normalized events, and stops the session. Adapters can also
  implement `runtime.RunAdapter` for native one-shot execution.
- Resident sessions: `runtime.Engine.StartSession`, `Prompt`, and `StopSession`
  keep an adapter process alive across turns, which is suitable for chat,
  support, and operations workflows.

ACP adapters expose the lower-level session metadata through
`runtime.SessionInspector`, including the underlying agent session id and
whether native resume was used. Applications can store that metadata in their
own schema and pass it back through `runtime.SessionRequest.ResumeSessionID`.

## Built-in Adapter Status

| Adapter | Runtime mode | Notes |
| --- | --- | --- |
| `hermes` | ACP resident and one-shot | Generates `HERMES_HOME/config.yaml` from `AgentSpec` and MCP server specs. |
| `open_code` | ACP resident and one-shot | Injects `OPENCODE_CONFIG_CONTENT`, isolated config dir, pure mode, and question tool support. |
| `claude_code` | ACP resident and one-shot | Uses `claude-agent-acp` as the default CLI and maps API token/base URL to environment variables. |
| `open_claw` | ACP resident and one-shot | Builds OpenClaw ACP bridge arguments for an existing gateway endpoint. Gateway lifecycle management belongs to the host application for now. |

These adapters provide SDK-level support and unit-tested configuration behavior.
Real CLI compatibility should still be validated by each host application in its
own environment, because installed CLI versions and protocol details can differ.
