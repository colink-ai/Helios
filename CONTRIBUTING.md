# Contributing

Helios is an AI application runtime SDK. Contributions should keep the core
runtime database-free and keep product-specific behavior in host applications.

## Development

```bash
go test ./...
```

Before changing adapter behavior, add or update tests in the adapter package and
consider adding a compatibility probe note in `docs/compatibility.md`.

## Design Rules

- Keep normalized contracts stable and application-oriented.
- Preserve raw protocol payloads when adding new semantic fields.
- Do not add database dependencies to the SDK.
- Prefer adapter-local protocol handling over core runtime special cases.
- Keep examples compile-ready without requiring real agent CLIs.

## Pull Request Checklist

- Tests pass with `go test ./...`.
- README or docs are updated for user-facing behavior.
- Compatibility impact is noted for built-in adapters.
- New runtime events preserve `schemaVersion`.
