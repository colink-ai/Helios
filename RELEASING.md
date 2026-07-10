# Releasing Helios

Helios uses semantic version tags for the Go module and records adapter
compatibility separately from the stable runtime event schema.

## Release Checklist

1. Confirm the worktree is clean and `main` is synchronized with its upstream.
2. Move user-visible entries from `Unreleased` into a dated version section in
   `CHANGELOG.md`.
3. Run the local SDK gates:

   ```bash
   go vet ./...
   go test ./...
   go test -race ./...
   ```

4. Run `cmd/helios-compat` against every adapter advertised by the release.
5. Run the integration build-tag suite for supported CLI, provider, and model
   combinations described in `docs/compatibility.md`.
6. Record the operating system, CLI versions, runtime config mode, scenarios,
   and compatibility output in the release notes.
7. Create an annotated semantic version tag only after all required checks pass:

   ```bash
   git tag -a vX.Y.Z -m "Helios vX.Y.Z"
   git push origin vX.Y.Z
   ```

## Required Compatibility Coverage

Each supported built-in adapter must pass capability detection, one-shot, and
resident-session scenarios. Resume, elicitation, permission, and multimodal
scenarios are required whenever the release claims those capabilities for that
adapter.

OpenClaw validation also requires a reachable Gateway. A failed probe caused by
a missing Gateway is an incomplete release check, not a passing adapter result.

Keep raw compatibility reports for protocol-drift investigations, but redact API
keys, gateway tokens, authorization headers, and secret-bearing environment
values before attaching reports to a public release.
