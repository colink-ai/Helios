# Changelog

## Unreleased

- Initialize Helios as an embeddable AI application runtime SDK.
- Add runtime contracts, session orchestration, event sinks, and session stores.
- Add ACP base adapter and built-in Hermes, OpenCode, Claude Code, and OpenClaw adapters.
- Add semantic event normalization for text, thinking, tool calls, questions,
  permissions, usage, plans, artifacts, handoffs, and errors.
- Add compatibility harness, CLI compatibility probe, diagnostics, resume
  helpers, file artifact store, and lightweight team runner.
- Add engine-level tool and elicitation result delivery.
- Make strict event sinks report streaming and asynchronous event failures.
- Return ACP prompt drain deadlines instead of partial successful results.
- Protect generated Hermes configuration with owner-only file permissions.
- Validate normal tests across Linux, macOS, and Windows, with vet and race
  detection as dedicated CI gates.
