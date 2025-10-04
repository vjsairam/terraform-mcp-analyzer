# Contributing

## Adding Rules
- Edit JSONL in `rules_samples/` or your own pack.
- Keep entries minimal; include docs URL and short excerpt.
- Run `go test ./...` and `make run.sample` to validate.

## Coding Standards
- Go 1.22, standard library first.
- Deterministic outputs; stable ordering in renderers and engine.
- No network calls in `scan` path.

## Releases
- Use `make pack.sample` to compress example packs.
- Attach packs to releases with cosign bundle (future milestone).

