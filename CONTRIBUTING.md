# Contributing to knet

Thanks for your interest in contributing! knet is a solo-started project that welcomes PRs of all sizes.

## Quick start

```bash
git clone https://github.com/themark007/knew.git
cd knew
go mod download
make build
./knet version
```

## Development workflow

1. **Fork** the repo and create a branch from `main`
2. Make your changes
3. Run `make fmt vet test` — all must pass
4. Open a PR against `main`

## Local testing

You need a running Kubernetes cluster. The easiest way:

```bash
# Start minikube
minikube start

# Deploy test workloads
kubectl create deployment nginx --image=nginx --replicas=2
kubectl expose deployment nginx --port=80 --type=ClusterIP

# Test your change
./knet scan
./knet graph full --format ascii
```

## Code style

- Run `gofmt -w .` before committing (`make fmt`)
- Follow standard Go conventions  
- Keep functions small and focused
- Add comments only where the logic isn't self-evident

## Adding a new command

1. Create `cmd/yourcommand.go`
2. Register it in the `init()` function with `rootCmd.AddCommand(yourCmd)`
3. Reuse `buildTopology()` from existing commands for k8s access
4. Support all standard output formats (`-o table/wide/json/yaml`)
5. Add entry to README.md

## Adding a new AI prompt

Add a `XxxPrompt(topo *k8s.Topology) string` function in `internal/ai/prompts.go` and wire it up in `cmd/analyze.go`.

## Releasing (maintainers only)

```bash
git tag v0.X.Y
git push origin v0.X.Y
# The GitHub Actions release workflow handles the rest
```

## Questions?

Open a [Discussion](https://github.com/themark007/knew/discussions) or file an issue.
