package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/themark007/knew/internal/ai"
	"github.com/themark007/knew/internal/config"
	"github.com/themark007/knew/internal/k8s"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage knet configuration",
	Long: `Manage knet's persistent configuration.

Settings are stored in ~/.config/knet/config.yaml.
AI keys are never required to be stored — you can pass them via flags or env vars.

Supported keys:
  openai_api_key      — OpenAI API key
  anthropic_api_key   — Anthropic API key
  openrouter_api_key  — OpenRouter API key
  default_namespace   — Default namespace (overrides kubeconfig)
  default_ai_provider — Default AI provider
  default_ai_model    — Default AI model

Examples:
  knet config set openai_api_key sk-...
  knet config set default_ai_provider openai
  knet config show`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Set(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("  ✓  %s = %s\n", args[0], maskSecret(args[0], args[1]))
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := config.ConfigDir()
		if err != nil {
			return err
		}
		fmt.Printf("  Config directory: %s\n\n", dir)

		keys := []struct{ key, label string }{
			{"openai_api_key", "OpenAI API key"},
			{"anthropic_api_key", "Anthropic API key"},
			{"openrouter_api_key", "OpenRouter API key"},
			{"ai_key", "Generic AI key (KNET_AI_KEY)"},
			{"default_namespace", "Default namespace"},
			{"default_ai_provider", "Default AI provider"},
			{"default_ai_model", "Default AI model"},
		}
		for _, k := range keys {
			v := viper.GetString(k.key)
			if v == "" {
				v = "(not set)"
			} else if isSecretKey(k.key) {
				v = maskSecret(k.key, v)
			}
			fmt.Printf("  %-30s  %s\n", k.label, v)
		}

		snap, _ := config.SnapshotsDir()
		fmt.Printf("\n  Snapshots directory: %s\n", snap)
		return nil
	},
}

var configTestCmd = &cobra.Command{
	Use:   "test-ai",
	Short: "Test AI provider connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		provName := aiProvider
		if provName == "" {
			provName = viper.GetString("default_ai_provider")
		}
		if provName == "" {
			return fmt.Errorf("no AI provider configured — use --ai-provider or set default_ai_provider")
		}
		key := aiKey
		if key == "" {
			key = config.GetAIKey(provName)
		}
		if key == "" {
			return fmt.Errorf("no API key configured for %s", provName)
		}
		prov, err := ai.New(ai.Config{Provider: provName, APIKey: key, Model: aiModel})
		if err != nil {
			return err
		}
		spin := startSpinner(fmt.Sprintf("Testing %s connection...", prov.Name()))
		resp, err := prov.Complete(context.Background(), "Reply with exactly: OK")
		spin.stop()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %v\n", prov.Name(), err)
			os.Exit(1)
		}
		fmt.Printf("  ✓  %s connected successfully\n", prov.Name())
		fmt.Printf("     Response: %s\n", resp)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configTestCmd)

	configTestCmd.Flags().StringVar(&aiProvider, "ai-provider", "", "provider to test: openai|anthropic|openrouter")
	configTestCmd.Flags().StringVar(&aiKey, "ai-key", "", "API key override")
	configTestCmd.Flags().StringVar(&aiModel, "ai-model", "", "model override")
}

func isSecretKey(key string) bool {
	return key == "openai_api_key" || key == "anthropic_api_key" ||
		key == "openrouter_api_key" || key == "ai_key"
}

func maskSecret(key, val string) string {
	if !isSecretKey(key) || len(val) < 8 {
		return val
	}
	return val[:4] + "..." + val[len(val)-4:]
}

// getAIKey is used by report.go
func getAIKey(provider string) string {
	return config.GetAIKey(provider)
}

// runAIAnalysis is a shared helper used by report.go
func runAIAnalysis(providerName, key, model, baseURL, mode string, topo *k8s.Topology) (string, error) {
	prov, err := ai.New(ai.Config{Provider: providerName, APIKey: key, Model: model, BaseURL: baseURL})
	if err != nil {
		return "", err
	}
	var prompt string
	switch mode {
	case "topology":
		prompt = ai.TopologyExplainPrompt(topo)
	case "policy":
		prompt = ai.PolicySuggestPrompt(topo)
	default:
		prompt = ai.SecurityAuditPrompt(topo)
	}
	return prov.Complete(context.Background(), prompt)
}
