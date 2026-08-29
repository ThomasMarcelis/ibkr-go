---
name: Bug report
about: Report a correctness or protocol-level issue
labels: bug
---

**Environment**
- Go version:
- `ibkr-go` version / commit:
- TWS or IB Gateway version and negotiated `server_version`:
- OS:

**Reproduction**
Minimal steps to reproduce. Include the exact calls made, connection parameters, and any relevant session state.

**Expected behavior**
What you expected to happen, and the docs section that describes it.

**Actual behavior**
What happened instead. If you can, attach a sanitized capture recorded with `cmd/ibkr-recorder` and checked with `cmd/ibkr-normalize -verify` (see `docs/transcripts.md`). Never attach a raw capture: it contains account data.

**Additional context**
Stack traces, wire-level observations, or related issues.
