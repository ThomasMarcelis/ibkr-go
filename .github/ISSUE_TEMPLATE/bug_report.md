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
What happened instead. If useful, attach a reviewed, sanitized transcript using the [capture promotion steps](https://github.com/ThomasMarcelis/ibkr-go/blob/main/docs/transcripts.md#promoting-a-capture-into-ci). `ibkr-normalize -verify` checks a capture; it does not create a sanitized attachment. Keep capture directories, `events.jsonl`, and `raw.txt` private: they contain account data.

**Additional context**
Stack traces, wire-level observations, or related issues.
