# Clean-Room Provenance Policy

## Purpose

`ibkr-go` is a clean-room implementation of a client for the Interactive Brokers TWS and IB Gateway socket protocol. The clean-room boundary exists so the library can be licensed, distributed, and consumed without the legal and design gravity of the official client source, and so its internal shape is free to be what Go idioms want rather than what the official API shape forces.

This policy is normative. Every contributor is expected to have read it before submitting a change that touches protocol-adjacent code.

## Allowed references

- Official Interactive Brokers protocol documentation.
- Wire and protocol behavior observed by the contributor against a TWS or IB Gateway instance the contributor controls (byte-level traces captured by the contributor).
- Other OSS Interactive Brokers libraries read for anti-pattern study only — to understand what to *not* copy, not as architectural templates.

## Forbidden sources

- Copying or paraphrasing source code from the official Interactive Brokers client libraries in any language.
- Borrowing the official library's package layout, module names, file structure, or naming conventions for the public API surface.
- Inheriting the official state machine structure, or the `EWrapper` / `EClient` callback shape, as the primary public design of this library.
- Copying code, types, or test fixtures from other OSS IB libraries, including `hadrianl/ibapi`, `scmhub/ibapi`, `scmhub/ibsync`, `portfoliome/ib`, or any similar project.

## Contributor review checklist

Before submitting a pull request that touches any protocol, wire, codec, session, or transport code, confirm:

- [ ] You have not read or referenced source code from the official Interactive Brokers client libraries while working on this change.
- [ ] You have not copied code or layout from any OSS IB library.
- [ ] Any reference to official protocol behavior is grounded in official documentation or wire traces you captured yourself.
- [ ] The PR body includes the clean-room attestation line from the pull request template.

## Reviewer responsibilities

Maintainers reviewing a pull request against protocol-adjacent code check that the clean-room attestation is present in the PR body, that the code does not mirror known official library shapes without reason, and that any new types or state machines are explicable from documentation or wire behavior alone. When a change cannot be justified without referencing official source, the PR is closed rather than merged.

## Initial commit

The initial commit of this repository carries the attestation `clean-room: no official TWS API source was referenced during this initial commit.` as the last line of its body. Every subsequent protocol-code commit carries the same attestation pattern, either in the body or in the PR description.
