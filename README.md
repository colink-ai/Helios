# Helios

Helios is an embeddable AI application runtime SDK for building agentic products.

It provides runtime contracts, normalized event streams, adapter interfaces, and
session orchestration primitives. It intentionally does not own application
storage, tenancy, identity, billing, or product-specific data models.

Helios sits between business applications and fast-moving foundation agents such
as Hermes, OpenCode, Claude Code, OpenClaw, and future ACP-compatible runtimes.
Its job is not to replace those agents. Its job is to make them safe, stable,
observable, and portable enough to embed in real products.

Business applications such as coding workspaces, operations assistants, customer
support copilots, security workbenches, and internal automation tools usually
want to own their own users, permissions, data, workflows, UI, audit trails, and
database schema. They should not also have to chase every CLI protocol change,
tool-call shape, session-resume detail, or streaming-output variant from every
agent they integrate. Helios absorbs that runtime integration churn behind a
stable semantic event layer.

## Design Principles

- Keep the SDK database-free. Applications persist sessions, messages, runs, and
  artifacts through their own stores.
- Normalize runtime events before they reach applications, so product code does
  not need to parse raw CLI or protocol output.
- Support embedded runtimes first, while leaving room for split worker
  deployments.
- Keep adapter interfaces stable enough for CLI agents, model runtimes, MCP
  tools, and future remote workers.
- Keep foundation-agent compatibility inside adapter packages. The core runtime
  event model should be small, stable, and application-oriented.
- Preserve raw protocol metadata alongside normalized fields so applications can
  adopt new agent capabilities without waiting for a core protocol change.

## Positioning

Helios is a runtime adapter layer for AI-native business applications.

It is valuable when an application needs to embed mature external agents but
still own product-level behavior: domain data, workflows, access control,
storage, audit, collaboration, and UI. In that shape, Helios provides:

- Process and session orchestration for embedded or split-worker deployments.
- A stable runtime API over multiple foundation agents.
- Normalized semantic events for messages, thinking, tool calls, questions,
  usage, artifacts, handoffs, plans, and errors.
- Adapter packages that track fast-moving agent protocols and CLI behavior.
- Optional persistence interfaces that let applications store runtime state in
  their own schema.

Helios is not a low-code application builder, a tenant platform, a product UI
framework, or a database abstraction. It is also not just a CLI wrapper. The
core value is the stable semantic layer between business applications and
changing agent runtimes.

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

## Quick Start

```go
registry := runtime.NewRegistry()
_ = all.Register(registry)

engine := runtime.NewEngine(registry, runtime.WithEventSink(runtime.EventSinkFunc(
    func(ctx context.Context, event contracts.RunEvent) error {
        // Persist events in the host application's own database.
        return nil
    },
)))

result, err := engine.Run(ctx, runtime.RunRequest{
    Agent: runtime.AgentSpec{
        Type:         "hermes",
        CLIPath:      "hermes",
        DefaultModel: "gpt-4.1",
    },
    Input: "Summarize this workspace.",
})
```

## Compatibility Probes

Use `runtime.CompatibilityHarness` to validate an installed CLI before enabling
it for users:

```go
harness := runtime.NewCompatibilityHarness(engine)
report := harness.Run(ctx, agent, []runtime.CompatibilityCheck{
    {Scenario: runtime.CompatDetect},
    {Scenario: runtime.CompatOneShot, Input: "Say hello"},
    {Scenario: runtime.CompatResident, Input: "Keep this session alive"},
})
```

The harness is intentionally SDK-level. It reports whether a runtime can be
detected, started, prompted, resumed, or asked for capabilities without requiring
Helios to know an application's database or tenant model.

For local CLI probes, use:

```bash
go run ./cmd/helios-compat -agent hermes -cli hermes
```

See [docs/compatibility.md](docs/compatibility.md) for the adapter matrix and
release-gate checklist.

## Examples

Compile-ready examples live under `examples/`:

- `examples/basic`: registry and event sink setup.
- `examples/permissions`: host approval callback shape.
- `examples/artifacts`: file artifact storage helper.

## Permission Flow

When an agent asks for permission, Helios emits a semantic event:

```go
if event.Type == contracts.EventPermissionAsked {
    permission := event.Chunk.Permission
    decision := runtime.PermissionDecision{Allow: true, Reason: "approved by policy"}
    _ = engine.SendPermissionResult(ctx, event.SessionID, permission.ID, decision)
}
```

Applications remain responsible for user policy, audit, and approval UI. Helios
only normalizes the runtime request and transports the decision back to the
adapter.

## Artifact Flow

Agents can emit `artifact.created` events. Applications may store artifacts in
their own systems, or use the SDK file helper:

```go
store := runtime.NewFileArtifactStore("/var/lib/my-app/runtime-artifacts")
saved, err := store.SaveArtifact(ctx, *event.Artifact)
data, err := store.ReadArtifact(ctx, saved)
```

The file helper keeps artifact paths under a configured root and does not create
or update application database rows.

## Session Resume

Host applications persist `runtime.SessionSnapshot` in their own schema. To
resume:

```go
snapshot, _ := appStore.LoadRuntimeSession(ctx, sessionID)
handle, err := engine.ResumeSessionFromSnapshot(ctx, *snapshot, agent)
```

The snapshot's `AgentSessionID` is passed to the adapter as
`ResumeSessionID`. ACP adapters try native `session/resume`, then `session/load`,
then fall back to `session/new` when necessary.

## Multi-Agent Teams

`runtime.TeamRunner` provides a lightweight WorkGraph runner for simple
agent-to-agent flows:

```go
runner := runtime.NewTeamRunner(engine)
teamResult, err := runner.Run(ctx, runtime.TeamRunRequest{
    Team:   team,
    Agents: agentSpecsByID,
    Input:  "Investigate this issue",
})
```

This is not a workflow platform. It is a small runtime primitive for sequential
agent teams, A2A message capture, and future handoff execution. WorkGraph edges
are used for deterministic ordering. Nodes can be skipped with
`metadata.condition` set to `skip`, `never`, `disabled`, or `false`; parallel
branches and joins are intentionally outside the current runner.

## Diagnostics

Applications can query session diagnostics for health pages or support tooling:

```go
diag, err := engine.Diagnostics(ctx, sessionID)
```

ACP diagnostics include the underlying agent session id, status, captured
stderr, resume strategy, and transport background errors when available.

## Versioning And Compatibility

Helios versions the application-facing semantic layer separately from individual
agent protocol details:

- Runtime events carry `schemaVersion`, currently `helios.semantic.v1`.
- Built-in adapter compatibility expectations are documented as
  `helios.adapters.v1`.
- Normalized fields are intended to evolve conservatively. New fields and event
  types may be added, but existing meanings should not change inside the same
  semantic version.
- Raw protocol payloads remain available through `Chunk.Raw`, `Chunk.Metadata`,
  and capability `Raw` fields so applications can adopt newly released agent
  behavior before Helios promotes it into stable semantic fields.
- Adapter packages are allowed to move faster than the core contracts because
  foundation agents and ACP-compatible CLIs evolve quickly.

For host applications, the recommended persistence key is:

```text
event.schemaVersion + event.type + event.sequence
```

Store raw payloads when auditability or forward compatibility matters.

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

## Project

- License: [MIT](LICENSE)
- Contributing guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
