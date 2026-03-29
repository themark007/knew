<div align="center">

# knet

**The missing Kubernetes network CLI.**  
Scan, visualize, audit, and AI-analyze your cluster network — in seconds, from your terminal.

[![CI](https://github.com/themark007/knew/actions/workflows/ci.yml/badge.svg)](https://github.com/themark007/knew/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/themark007/knew?color=blue&label=release)](https://github.com/themark007/knew/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/themark007/knew)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Stars](https://img.shields.io/github/stars/themark007/knew?style=social)](https://github.com/themark007/knew/stargazers)

</div>

---

## Why knet?

When something in your cluster stops talking to something else, you need answers fast.

`kubectl` shows you objects. `knet` shows you **why traffic flows — or doesn't**.

| Without knet | With knet |
|---|---|
| Stare at 50 NetworkPolicy YAMLs | `knet check --from ns/a --to ns/b --port 8080` |
| Draw topology by hand | `knet graph full --format tui` |
| Guess which namespaces have no isolation | `knet audit -A` |
| Write GPT prompts manually | `knet analyze security --ai-provider openai` |

```
knet scan -A
knet graph full --format tui
knet check --from default/frontend --to default/backend --port 8080
knet audit -A
knet analyze security --ai-provider openai
```

---

## Installation

### One-liner (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/themark007/knew/main/scripts/install.sh | bash
```

### Homebrew

```bash
brew tap themark007/knet
brew install knet
```

### Direct download

Grab the [latest release](https://github.com/themark007/knew/releases/latest):

| Platform | File |
|---|---|
| macOS Apple Silicon | `knet_darwin_arm64.tar.gz` |
| macOS Intel | `knet_darwin_x86_64.tar.gz` |
| Linux amd64 | `knet_linux_x86_64.tar.gz` |
| Linux arm64 | `knet_linux_arm64.tar.gz` |
| Windows amd64 | `knet_windows_x86_64.zip` |

> **No Go installation required.** Binaries are statically linked.

```bash
knet version
# knet version 0.1.0
```

---

## Quick start

```bash
knet scan                                           # scan current namespace
knet scan -A --include-policies --include-ingress   # everything, everywhere
knet graph full --format tui                        # interactive topology TUI
knet check --from default/frontend --to default/backend --port 8080
knet trace --from default/frontend --to default/backend --port 8080
knet audit -A                                       # namespace isolation gaps
knet report --output-file report.html -A            # HTML report
knet analyze security --ai-provider openai          # AI security review
```

---

## Feature tour

<details>
<summary><b>📡 Scan</b> — pods, services, policies, ingresses</summary>

```bash
knet scan
knet scan -A --include-policies --include-ingress
knet scan -n production -o json | jq '.pods[].name'
knet scan -l app=frontend -o yaml
```

</details>

<details>
<summary><b>🗺️ Graph</b> — ASCII · TUI · DOT · Mermaid</summary>

```bash
knet graph full --format tui     # arrow keys to navigate, / to filter, q to quit
knet graph full --format ascii
knet graph full --format dot | dot -Tsvg -o topology.svg
knet graph full --format mermaid
```

Example ASCII output:

```
[INGRESS]
  nginx-ingress (default)

[SERVICES]
  frontend-svc ──> backend-svc

[PODS]
  frontend-abc12 ──> backend-xyz78
  monitoring-pod
```

</details>

<details>
<summary><b>🔍 Check &amp; Trace</b> — static NetworkPolicy analysis</summary>

```bash
knet check --from default/frontend --to default/backend --port 8080
# ✓ ALLOWED  (exit 0)

kubectl apply -f deny-all.yaml

knet check --from default/frontend --to default/backend --port 8080
# ✗ BLOCKED  (exit 1)

knet trace --from default/frontend --to default/backend --port 8080
# Step 1: Resolved source pods: [frontend-abc123]
# Step 2: Resolved destination pods: [backend-xyz789]
# Step 3: Egress on source: no policy → allow all
# Step 4: Ingress on destination: deny-all matched
# Step 5: No allow rule for port 8080 from source
# Result: BLOCKED
```

Works **offline** — pure static NetworkPolicy evaluation, no network calls.

</details>

<details>
<summary><b>🛡️ Audit</b> — namespace isolation coverage</summary>

```bash
knet audit -A

# NAMESPACE      PODS   POLICIES   COVERAGE
# default        4      0          none       ← exposed
# production     8      5          full       ✓
# staging        6      2          partial    ⚠
# monitoring     3      0          none       ← exposed
```

Coverage levels:

- `none` — no NetworkPolicies, all traffic allowed by default
- `partial` — some pods covered, some not
- `full` — every pod has at least one NetworkPolicy

</details>

<details>
<summary><b>📸 Diff</b> — snapshot &amp; compare topology changes</summary>

```bash
knet diff save --name pre-deploy -A
kubectl apply -f new-version/
knet diff --snapshot pre-deploy -A
# + pods added:     api-v2-xxx
# - pods removed:   api-v1-yyy
# ~ policies changed: frontend-policy (added port 9000)
```

Snapshots stored at `~/.config/knet/snapshots/`.

</details>

<details>
<summary><b>👁️ Watch</b> — live refreshing TUI</summary>

```bash
knet watch --interval 5 -A
# p — pause/resume   r — force refresh   q — quit
```

</details>

<details>
<summary><b>🤖 AI Analysis</b> — OpenAI · Anthropic · OpenRouter</summary>

```bash
knet analyze security  --ai-provider openai
knet analyze topology  --ai-provider anthropic -A
knet analyze policy    --ai-provider openrouter --ai-model mistralai/mistral-large
knet analyze generate  --ai-provider openai \
  --description "Allow frontend to reach backend on port 8080 only"
```

Set keys once:

```bash
knet config set ai.provider openai
knet config set ai.key sk-...
```

</details>

<details>
<summary><b>📄 HTML Report</b> — self-contained, shareable, air-gap safe</summary>

```bash
knet report --output-file cluster-report.html -A
knet report --output-file cluster-report.html -A --ai-provider openai --ai-mode security
open cluster-report.html
```

Single HTML file — no CDN, no internet required.

</details>

---

## Commands reference

| Command | Description |
|---|---|
| `knet scan` | Scan pods, services, policies, ingresses |
| `knet pods` | List pods |
| `knet services` / `svc` | List services |
| `knet policies` / `netpol` | List NetworkPolicies |
| `knet graph [pods\|services\|full]` | Visualize topology |
| `knet check` | Connectivity check (ALLOWED / BLOCKED, exit 0/1) |
| `knet trace` | Step-by-step policy trace |
| `knet audit` | Namespace isolation audit |
| `knet diff save` / `knet diff` | Snapshot and compare topology |
| `knet watch` | Live-refreshing TUI |
| `knet analyze` | AI-powered analysis |
| `knet report` | Generate HTML report |
| `knet config` | Manage configuration |
| `knet version` | Print version |

### Global flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--namespace` | `-n` | current | Target namespace |
| `--all-namespaces` | `-A` | false | All namespaces |
| `--output` | `-o` | `table` | `table` · `wide` · `json` · `yaml` |
| `--context` | | current | kubeconfig context |
| `--kubeconfig` | | `~/.kube/config` | kubeconfig path |
| `--selector` | `-l` | | Label selector |
| `--timeout` | | `30s` | API timeout |
| `--no-color` | | false | Disable color |

---

## AI providers

| Provider | Flag | Env var | Default model |
|---|---|---|---|
| OpenAI | `--ai-provider openai` | `OPENAI_API_KEY` | `gpt-4o` |
| Anthropic | `--ai-provider anthropic` | `ANTHROPIC_API_KEY` | `claude-3-5-sonnet-20241022` |
| OpenRouter | `--ai-provider openrouter` | `OPENROUTER_API_KEY` | `openai/gpt-4o` |

AI is **entirely optional** — all other features work with no key.

---

## Output formats

```bash
knet scan -o json  | jq '.pods[] | select(.phase=="Running")'
knet scan -o yaml  > snapshot.yaml
knet audit -A -o json | jq '.[] | select(.coverage=="none")'
```

---

## Configuration

```bash
knet config set ai.provider openai
knet config set ai.key sk-...
knet config show
knet config test-ai
```

Config file: `~/.config/knet/config.yaml`

---

## Building from source

Requires Go 1.22+.

```bash
git clone https://github.com/themark007/knew.git
cd knew
make build
./knet version
```

| Make target | Action |
|---|---|
| `make build` | Compile binary |
| `make install` | Install to `/usr/local/bin` |
| `make test` | Run tests |
| `make fmt` | Format code |
| `make vet` | Run go vet |
| `make snapshot` | Multi-platform build (needs goreleaser) |
| `make release` | Publish GitHub release |

---

## Contributing

Contributions are very welcome — bug fixes, new features, better docs, anything!

See [CONTRIBUTING.md](CONTRIBUTING.md) to get started.  
Check [open issues](https://github.com/themark007/knew/issues) to find something to work on.

---

## Star history

[![Star History](https://api.star-history.com/svg?repos=themark007/knew&type=Date)](https://star-history.com/#themark007/knew&Date)

---

## License

[MIT](LICENSE) — free to use, modify, and distribute.

---

<div align="center">

**If knet saved you time, a ⭐ helps others discover it — thank you!**

[Report a bug](https://github.com/themark007/knew/issues/new?template=bug_report.yml) · [Request a feature](https://github.com/themark007/knew/issues/new?template=feature_request.yml) · [Discussions](https://github.com/themark007/knew/discussions)

</div>
