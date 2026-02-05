# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly. **Do not open a public GitHub issue.**

Email **hello@sendrec.eu** with:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if you have one)

You will receive an acknowledgment within 48 hours. We aim to provide a fix or mitigation plan within 7 days for critical issues.

## Scope

This policy covers the SendRec application:

- Go backend API (`internal/`, `cmd/`)
- React frontend (`web/`)
- Docker Compose deployment configuration
- Database migrations (`migrations/`)

## Supported Versions

| Version | Supported |
| ------- | --------- |
| main    | Yes       |

We currently support only the latest code on the `main` branch. As we introduce releases, this table will be updated.

## Recognition

We appreciate responsible disclosure and will credit reporters in the release notes (unless you prefer to remain anonymous).
