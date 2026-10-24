# Contributing to KyRecovery

[Product overview](README.md) · [Operations](docs/OPERATIONS.md) · [Assurance case](docs/ASSURANCE-CASE.md)

## Reporting bugs and asking for features

Open a [GitHub issue](https://github.com/Busnes-app/KyRecovery-server/issues). Say which
version you run (`kyrecovery version`, or the image tag), what you did, what you expected and
what happened. Leave secrets, tokens and capsule contents out of the issue.

**Security problems are not issues.** Report them privately through the repository's
**Security** tab → **Report a vulnerability**, as the
[Busnes-app security policy](https://github.com/Busnes-app/.github/blob/main/SECURITY.md)
describes.

## Making a change

1. Branch from `main` and open a pull request against `main`. `main` takes changes only through
   pull requests.
2. Read [AGENTS.md](AGENTS.md) first. Its security invariants are binding, and a change that
   alters behaviour, responsibilities or verification updates the nearest AGENTS.md and the
   owning guide under `docs/` in the same pull request.
3. The `go`, `security` and `docker` CI jobs must pass before a pull request can merge. Run the
   checks in [Operations → Verify](docs/OPERATIONS.md#verify) locally first.
4. Describe what changed and how you checked it in the pull request.

## Tests

New behaviour comes with a test that exercises it, and non-trivial logic includes at least one
runnable check (a unit test or a minimal self-check). A security change also needs a test for the
attack it stops, not only the happy path; the tests named in the
[assurance case](docs/ASSURANCE-CASE.md#common-weaknesses-and-how-they-are-countered) show the
pattern. CI runs `go test -race -count=1 ./...` on every push and pull request.

## Coding standard

Contributions follow these style guides. CI enforces the parts a tool can check, so a style
failure is a red build, not a review comment.

| Language | Style guide | Enforced by |
|---|---|---|
| Go | [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) | `gofmt -l` (must print nothing), `go mod tidy` with no diff, and `go vet ./...` in the `go` job |
| Shared browser UI (`internal/server/static/ky-ui/`) | Generated from Busnes-app/ky-ui; never edited here | `node internal/server/static/ky-ui/check-vendor.mjs` in the `go` job |
| Dashboard styles | Text colours reach a 4.5:1 contrast ratio in light and dark themes (WCAG AA) | `node scripts/check-contrast.mjs` in the `go` job |

The dashboard's own JavaScript has no linter; keep to the style of the file you are editing, and
escape every interpolated value with `esc` or `escJs` (`TestEmbeddedDashboardEscapesEveryInnerHTMLSink`
fails otherwise). Prefer the standard library and the shared `ky-primitives` packages to new
dependencies.
