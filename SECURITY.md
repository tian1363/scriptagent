# Security Policy

## Supported version

Security fixes are applied to the latest commit on the `main` branch. This project is currently in beta and has not completed an independent security audit.

## Reporting a vulnerability

Please do not disclose a suspected vulnerability in a public issue. Use GitHub's private vulnerability reporting feature for this repository and include:

- affected version or commit;
- reproduction steps;
- expected impact;
- suggested mitigation, if available.

Do not include real API keys, user content or personal data in the report. The maintainer will acknowledge a complete report, investigate it and coordinate disclosure after a fix is available.

## Deployment responsibilities

Public deployments must use HTTPS, encrypted API-key storage, invite-only registration, restricted filesystem permissions and regular backups. The default SQLite and local-file configuration is intended for a single application instance.

Authentication rate limits are currently kept in process memory. Public or multi-instance deployments require a shared rate-limit store plus verified-email and password-recovery flows.
