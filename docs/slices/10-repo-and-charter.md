# Slice 10: `ibkr-go` Repo And Charter

> **Status: complete.** Initial commit `502be28` on `main` at `git@github.com:ThomasMarcelis/ibkr-go.git`. This file is kept for historical reference. See `IMPLEMENTATION_STATUS.md` for the authoritative current state.

## Objective

Create the standalone `ibkr-go` repo and ship it as a production-grade OSS Go library on day 1. Lock the clean-room project charter, the public API direction, and the testing philosophy. No protocol implementation yet — that lands in slice `11` — but everything around the code must be finished, serious, and ready for outside contributors or consumers to take seriously.

## Why This Slice Exists

An ibkr-go project charter depends on a real repository, not a hypothetical future dependency. A charter-only repo with a stub README and no license is not enough: a serious OSS Go library is judged within 30 seconds of landing on its GitHub page. License, README, CI, contribution rules, and the testing philosophy set the quality bar the rest of the project will hold itself to.

This slice locks that bar before any protocol code exists. Slice `11` then fills in the implementation inside a repo that already looks, reads, and tests like an S-tier library.

## Repo

- `ibkr-go` (standalone OSS)

## Decisions Already Made

- Standalone repo at `git@github.com:ThomasMarcelis/ibkr-go.git`, pre-created as private and empty by the user. Initial population pushed via `git init -b main` + `git push -u origin main`.
- **License: MIT.** Visibility is currently private; user flips to public manually when ready. Public-facing docs are written as if the repo were public, so no content update is needed at the visibility flip.
- **CI: GitHub Actions** (`.github/workflows/ci.yml`), runs `gofmt -l .`, `go vet`, `go build`, `golangci-lint run`, `go test ./...` on push and PR. Must be green on day 1 even though there is no protocol code yet.
- Go 1.26, zero non-stdlib dependencies at day 1.
- TWS / IB Gateway first, read-only-first v1, radically new public API, strict clean-room implementation.
- Public root package is `ibkr`.
- Internal package layout: `ibkr/`, `internal/{wire,codec,transport,session}/`, `testing/testhost/`, `experimental/raw/`.
- Commit convention: see `AGENTS.md` §Commit Convention.
- Testing philosophy: see `AGENTS.md` §Testing Philosophy. Normative, not aspirational.

## Deliverable (shipped)

- `LICENSE` (MIT, copyright Thomas Marcelis 2026), `README.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` (Contributor Covenant v2.1 verbatim), `SECURITY.md`, `AGENTS.md`, `.gitignore`, `.golangci.yml`, `go.mod` (zero deps).
- `.github/workflows/ci.yml` (green on initial push), `.github/ISSUE_TEMPLATE/{bug_report,feature_request}.md`, `.github/PULL_REQUEST_TEMPLATE.md`.
- `docs/provenance.md` (clean-room policy), `docs/anti-patterns.md`, `docs/feature-matrix.md`, `docs/roadmap.md`.
- Empty package skeleton: `ibkr/doc.go`, `internal/{wire,codec,transport,session}/doc.go`, `testing/testhost/doc.go`, `experimental/raw/doc.go`.

## Non-Goals

- **Protocol implementation** — all Go code under `ibkr/`, `internal/*`, `testing/testhost/`, and `experimental/raw/` is empty package declarations only. Slice 11 owns the implementation.
- Dogfooding integration — handled by external consumers.
- `compat/eclient/` bridge — explicitly deferred until after legal review.
- Scanners, news, Flex, Client Portal Web API — out of v1 scope entirely.

## Dependencies

- none

## Write Scope

- entire `ibkr-go` repo (initial population)

## Required Tests

- Day-1 CI (`gofmt -l .`, `go vet ./...`, `go build ./...`, `golangci-lint run`, `go test ./...`) must pass on the initial push.
- `go test ./...` runs zero actual tests at day 1.

## Definition Of Done

- `git@github.com:ThomasMarcelis/ibkr-go.git` has the initial commit pushed, default branch `main`. Repo remains private (user flips visibility manually later).
- Every file in the Deliverable section is present and correct.
- `LICENSE` is MIT with correct copyright holder and year.
- `AGENTS.md` contains the core mindset, Go style, build-by-specification methodology, and testing philosophy sections.
- `.github/workflows/ci.yml` passes green on the initial push.
- Package skeleton directories exist with empty `doc.go` files.
- Initial commit follows the commit convention.
- Clean-room attestation line present in the initial commit body.
