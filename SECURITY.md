# Security Policy

## Supported Versions

Security fixes are released for the latest tagged minor line. The current
supported line is v1.6. Superseded minor lines receive no feature or security
backports unless a release advisory explicitly says otherwise.

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

Do not include real credentials, live account identifiers, or production wire traces in public issues or PRs. Scrub them before sharing.

## Capture Handling

Never attach unsanitized Gateway/TWS traffic to a public report. Use the
repository capture and normalization tools, review the normalized output, and
include the Gateway/TWS build, negotiated `server_version`, scenario, and
SHA-256 of the private source capture. Account IDs, order identifiers, host
details, and personally identifying fields must be redacted before sharing.
