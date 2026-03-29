package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/themark007/knew/internal/ai"
	"github.com/themark007/knew/internal/config"
	"github.com/themark007/knew/internal/k8s"
	"github.com/spf13/cobra"
)

var (
	aiProvider string
	aiKey      string
	aiModel    string
	aiBaseURL  string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [security|topology|policy|generate]",
	Short: "AI-powered network analysis",
	Long: `Use an AI provider to analyze your Kubernetes network topology.

Subcommands:
  security   — audit for security issues and get a risk rating
  topology   — plain-language explanation of network topology
  policy     — suggest NetworkPolicy improvements
  generate   — generate a NetworkPolicy YAML from a description

AI Providers:
  openai      — OpenAI GPT-4o (default model)
  anthropic   — Anthropic Claude 3.5 Sonnet (default model)
  openrouter  — OpenRouter (any model via --ai-model)

API keys can also be set via environment variables:
  OPENAI_API_KEY, ANTHROPIC_API_KEY, OPENROUTER_API_KEY

Examples:
  knet analyze security --ai-provider openai --ai-key $OPENAI_API_KEY
  knet analyze topology --ai-provider anthropic
  knet analyze policy --ai-provider openrouter --ai-model mistralai/mistral-7b-instruct
  knet analyze generate "allow frontend to reach backend on port 8080" --ai-provider openai`,
	ValidArgs: []string{"security", "topology", "policy", "generate"},
	RunE:      runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringVar(&aiProvider, "ai-provider", "", "AI provider: openai|anthropic|openrouter")
	analyzeCmd.Flags().StringVar(&aiKey, "ai-key", "", "API key for the AI provider (or use env var)")
	analyzeCmd.Flags().StringVar(&aiModel, "ai-model", "", "model to use (provider-specific default if empty)")
	analyzeCmd.Flags().StringVar(&aiBaseURL, "ai-base-url", "", "custom base URL (for OpenAI-compatible endpoints)")
	_ = analyzeCmd.MarkFlagRequired("ai-provider")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	subCmd := "security"
	if len(args) > 0 {
		subCmd = args[0]
	}

	key := aiKey
	if key == "" {
		key = config.GetAIKey(aiProvider)
	}
	if key == "" {
		return fmt.Errorf("no API key provided — use --ai-key or set %s_API_KEY environment variable",
			envKeyName(aiProvider))
	}

	provider, err := ai.New(ai.Config{
		Provider: aiProvider,
		APIKey:   key,
		Model:    aiModel,
		BaseURL:  aiBaseURL,
	})
	if err != nil {
		return err
	}

	cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}
	ns := namespace
	if ns == "" {
		ns = defaultNS
	}

	spin := startSpinner("Scanning cluster...")
	topo, err := k8s.BuildTopology(k8s.BuildOptions{
		Clientset:       cs,
		Namespace:       ns,
		AllNamespaces:   allNs,
		LabelSelector:   labelSel,
		Timeout:         timeout,
		IncludePolicies: true,
		IncludeIngress:  true,
	})
	spin.stop()
	if err != nil {
		return err
	}

	var prompt string
	switch subCmd {
	case "security":
		prompt = ai.SecurityAuditPrompt(topo)
	case "topology":
		prompt = ai.TopologyExplainPrompt(topo)
	case "policy":
		prompt = ai.PolicySuggestPrompt(topo)
	case "generate":
		if len(args) < 2 {
			return fmt.Errorf("generate requires a description argument, e.g.:\n  knet analyze generate \"allow frontend to reach backend on port 8080\"")
		}
		prompt = ai.GeneratePolicyPrompt(args[1], topo)
	default:
		return fmt.Errorf("unknown subcommand %q — use: security, topology, policy, generate", subCmd)
	}

	spin2 := startSpinner(fmt.Sprintf("Calling %s...", provider.Name()))
	response, err := provider.Complete(context.Background(), prompt)
	spin2.stop()
	if err != nil {
		return fmt.Errorf("AI request failed: %w", err)
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "────────────────────────────────────────────────────────────────")
	fmt.Fprintf(os.Stdout, "  knet analyze %s  [%s]\n", subCmd, provider.Name())
	fmt.Fprintln(os.Stdout, "────────────────────────────────────────────────────────────────")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, response)
	fmt.Fprintln(os.Stdout)
	return nil
}

func envKeyName(provider string) string {
	switch provider {
	case "openai":
		return "OPENAI"
	case "anthropic":
		return "ANTHROPIC"
	case "openrouter":
		return "OPENROUTER"
	default:
		return "KNET_AI"
	}
}
