# Security Policy

## Supported Versions

| Version | Support |
|---|---|
| v2, latest tagged release | Security fixes |
| v1 | Unsupported (deprecated) |

Security fixes ship on the latest tagged v2 release. Older v2 releases and v1
receive no backports unless a release advisory explicitly says otherwise.

Use the latest Go 1.26 patch release. The minimum language version does not
imply that older, vulnerable toolchain patch releases are supported.

## Reporting a Vulnerability

Do not open a public issue for security-sensitive reports.

Use GitHub's private vulnerability reporting:
<https://github.com/ThomasMarcelis/ibkr-go/security/advisories/new>

Email fallback: <thomasmarcelis@gmail.com>

Include reproduction steps, affected versions, and your disclosure timeline.
Expect acknowledgement within three business days and an initial remediation
assessment within ten business days.

## Capture Handling

Never attach unsanitized Gateway or TWS traffic to a public report. Record
with `cmd/ibkr-recorder`, normalize with `cmd/ibkr-normalize`, review the
output, and include the Gateway/TWS build, negotiated `server_version`,
scenario, and the SHA-256 of the private source capture. Redact account IDs,
order identifiers, host details, and personally identifying fields before
sharing. See [`docs/transcripts.md`](docs/transcripts.md).
