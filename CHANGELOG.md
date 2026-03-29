# Changelog

All notable changes to knet are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

---

## [0.1.0] — 2026-03-29

### Added
- `knet scan` — scan pods, services, NetworkPolicies, Ingresses
- `knet pods` / `knet services` / `knet policies` — focused resource listing
- `knet graph` — network topology as ASCII art, interactive TUI, Graphviz DOT, or Mermaid
- `knet check` — static NetworkPolicy connectivity analysis (ALLOWED / BLOCKED)
- `knet trace` — step-by-step trace through NetworkPolicy decision chain
- `knet audit` — per-namespace isolation coverage audit
- `knet diff save` / `knet diff` — snapshot topology and compare changes
- `knet watch` — live auto-refreshing TUI
- `knet analyze` — AI-powered security audit, topology explanation, policy suggestions, policy generation (OpenAI, Anthropic, OpenRouter)
- `knet report` — self-contained HTML report with optional AI analysis
- `knet config` — persistent configuration management
- `knet version` — version info
- Multi-platform distribution: macOS, Linux, Windows × amd64/arm64
- Shell installer (`scripts/install.sh`)
- Homebrew formula (`Formula/knet.rb`)
- GoReleaser configuration for automated releases
- GitHub Actions CI (build, test, lint, cross-compile) and release workflows

[Unreleased]: https://github.com/themark007/knew/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/themark007/knew/releases/tag/v0.1.0
