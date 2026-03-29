# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| latest  | ✅ |
| < latest | ❌ (please upgrade) |

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Email: **security@knet.dev** (or open a [private security advisory](https://github.com/themark007/knew/security/advisories/new) on GitHub).

Include:
- A description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (optional)

We aim to acknowledge reports within **48 hours** and release a patch within **7 days** for critical issues.

## Scope

knet is a read-only CLI tool that uses your existing kubeconfig credentials. It does not:
- Write to the cluster
- Store credentials anywhere except `~/.config/knet/config.yaml` (which you control)
- Make outbound network calls except to your configured AI provider

The main security surface areas are:
- **AI API key handling** — keys are only sent to the provider you configure
- **kubeconfig parsing** — uses the upstream `k8s.io/client-go` library
- **HTML report output** — uses `html/template` with auto-escaping to prevent XSS
