# Security Policy

## Reporting a Vulnerability

Do not open a public issue for security-sensitive reports.

Use GitHub's private vulnerability reporting:
<https://github.com/ThomasMarcelis/ibkr-go/security/advisories/new>

Email fallback: <thomasmarcelis@gmail.com>

Include reproduction steps, affected versions, and your disclosure timeline. You will receive an acknowledgement within a reasonable time and a remediation plan once the issue is understood.

Do not include real credentials, live account identifiers, production wire
traces, SDK callback captures, SDK-event fixtures, order IDs, execution IDs, or
account-specific metadata in public issues or PRs unless they have been
sanitized. Scrub them before sharing.

Live trading verification for this repository is paper/sandbox only. Never
include reproduction steps that require real-account order placement, option
exercise, FA writes, or other account-mutating actions.
