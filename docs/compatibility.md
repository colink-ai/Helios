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

## Real CLI Integration Tests

The SDK unit tests use fake ACP processes so `go test ./...` remains stable and
does not require credentials or network access. To validate an installed
foundation-agent CLI with a real API key, run the optional integration suite:

```bash
HELIOS_INTEGRATION=1 \
HELIOS_AGENT_TYPE=open_code \
HELIOS_AGENT_CLI=opencode \
HELIOS_API_URL=https://model.example/v1 \
HELIOS_API_KEY=... \
HELIOS_MODEL=gpt-4.1 \
go test -tags=integration ./integration
```

Useful environment variables:

| Variable | Meaning |
| --- | --- |
| `HELIOS_AGENT_TYPE` | Built-in adapter type: `hermes`, `open_code`, `claude_code`, or `open_claw`. Defaults to `open_code`. |
| `HELIOS_AGENT_CLI` | CLI executable path. Defaults to the adapter's common CLI name. |
| `HELIOS_API_URL` | Optional model API base URL passed through `runtime.AgentSpec.APIURL`. |
| `HELIOS_API_KEY` | API key passed through `runtime.AgentSpec.APIToken`. Required unless `HELIOS_ALLOW_EXISTING_AUTH=1`. |
| `HELIOS_MODEL` | Optional default model name. |
| `HELIOS_PROMPT` | Prompt sent to the real agent. Defaults to asking for `helios-ok`. |
| `HELIOS_EXPECT_CONTAINS` | Expected substring in the real model response. Defaults to `helios-ok`; set empty to only require non-empty output. |
| `HELIOS_RUN_MULTIMODAL` | Set to `1` to validate image input with an in-memory red PNG. |
| `HELIOS_MULTIMODAL_PROMPT` | Prompt used for the multimodal test. Defaults to asking for the image color. |
| `HELIOS_MULTIMODAL_EXPECT_CONTAINS` | Expected substring for the multimodal response. Defaults to `red`. |
| `HELIOS_EXPECT_MULTIMODAL_FAILURE` | Set to `1` when validating a model that should fail or not satisfy the visual task. |
| `HELIOS_TIMEOUT_SECONDS` | End-to-end timeout. Defaults to 120 seconds. |
| `HELIOS_WORKDIR` | Optional working directory for the agent process. Defaults to a temporary directory. |
| `HELIOS_RUNTIME_HOME` | Optional runtime home/config directory. Defaults to a temporary directory. |
| `HELIOS_RUN_ONESHOT` | Set to `1` to also validate `Engine.Run`; resident session validation always runs. |
| `HELIOS_ALLOW_EXISTING_AUTH` | Set to `1` when the CLI should use existing local auth instead of `HELIOS_API_KEY`. |

To cover built-in foundation-agent adapters across text and multimodal model
settings, enable the agent coverage suite:

```bash
HELIOS_INTEGRATION=1 \
HELIOS_RUN_AGENT_COVERAGE=1 \
HELIOS_OPENAI_API_URL=https://model.example/v1 \
HELIOS_ANTHROPIC_API_URL=https://model.example/anthropic \
HELIOS_API_KEY=... \
HELIOS_TEXT_MODEL=glm-5 \
HELIOS_MULTIMODAL_MODEL=qwen3.7-plus \
HELIOS_TEXT_ONLY_MODEL=glm-5 \
go test -tags=integration ./integration -run TestRealAgentCLIAgentCoverage
```

By default this suite covers `hermes`, `open_code`, `claude_code`, and
`open_claw`. For each adapter it runs the same contract:

- `text`: the adapter must start a resident session and return the expected
  text using `HELIOS_TEXT_MODEL`.
- `multimodal`: when `HELIOS_MULTIMODAL_MODEL` is set, the adapter must deliver
  the in-memory PNG to the agent and return the expected visual answer.
- `multimodal_fail`: when `HELIOS_TEXT_ONLY_MODEL` is set, the same image prompt
  must fail or avoid satisfying the visual assertion. This is only valid for
  models that do not support image input.

Override the generated scenario list with
`HELIOS_AGENT_COVERAGE_SCENARIOS` when a release needs a narrower or different
set:

```bash
HELIOS_AGENT_COVERAGE_SCENARIOS='hermes_text:hermes:openai:glm-5:text,hermes_vision:hermes:openai:qwen3.7-plus:multimodal'
```

Each scenario is `name:agent:protocol:model:mode`, where `protocol` is
`openai` or `anthropic`, and `mode` is `text`, `multimodal`, or
`multimodal_fail`. `text` and `multimodal` must succeed; `multimodal_fail`
is only for models that do not support image input. Per-agent CLI overrides use
variables such as
`HELIOS_HERMES_CLI`, `HELIOS_OPEN_CODE_CLI`, and `HELIOS_CLAUDE_CODE_CLI`.

OpenClaw's ACP CLI depends on a running OpenClaw Gateway. Point the integration
suite at that gateway with `HELIOS_OPEN_CLAW_GATEWAY_URL`, or with
`HELIOS_OPEN_CLAW_GATEWAY_PORT` when using the default local websocket URL.
If the gateway requires a bridge token, provide it with
`HELIOS_OPEN_CLAW_GATEWAY_TOKEN`; do not write that token into repository files.

These tests are not included in default coverage numbers. They are release or
environment checks for real CLI installation, credential wiring, network access,
and model-provider behavior.

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
3. Run the `integration` build-tag tests for every real CLI/API-key combination
   that should be release-supported.
4. Record CLI versions and probe output in release notes.
5. Keep raw compatibility output when investigating protocol drift.
