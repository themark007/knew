package cmd

import (
	"fmt"
	"os"

	"github.com/themark007/knew/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global flag values shared by all subcommands.
var (
	cfgFile     string
	kubeconfig  string
	kubeContext string
	namespace   string
	allNs       bool
	outputFmt   string
	verbose     bool
	timeout     int
	labelSel    string
	noColor     bool
)

var rootCmd = &cobra.Command{
	Use:   "knet",
	Short: "Kubernetes network scanner and analyzer",
	Long: `knet scans and visualizes Kubernetes cluster networks.

Features:
  • Scan pods, services, NetworkPolicies and Ingresses
  • Render network topology as ASCII art or interactive TUI
  • Check pod-to-pod connectivity through static NetworkPolicy analysis
  • Trace why traffic is allowed or blocked step-by-step
  • Audit namespaces for isolation gaps
  • Compare topology snapshots (diff)
  • Live watch mode with auto-refresh
  • AI-powered analysis (OpenAI, Anthropic, OpenRouter)
  • Export self-contained HTML reports

Quick start:
  knet scan                      # scan current namespace
  knet scan -A                   # scan all namespaces
  knet graph full --format tui   # interactive graph
  knet check --from ns/pod-a --to ns/pod-b --port 80
  knet analyze security --ai-provider openai --ai-key $OPENAI_API_KEY
  knet report --output-file report.html`,
	SilenceUsage: true,
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfgFile, "config", "", "config file (default: ~/.config/knet/config.yaml)")
	pf.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	pf.StringVar(&kubeContext, "context", "", "kubeconfig context to use")
	pf.StringVarP(&namespace, "namespace", "n", "", "namespace to scan (default: current context namespace)")
	pf.BoolVarP(&allNs, "all-namespaces", "A", false, "scan all namespaces")
	pf.StringVarP(&outputFmt, "output", "o", "table", "output format: table|wide|json|yaml")
	pf.BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
	pf.IntVar(&timeout, "timeout", 30, "timeout in seconds for Kubernetes API calls")
	pf.StringVarP(&labelSel, "selector", "l", "", "label selector to filter resources (e.g. app=nginx)")
	pf.BoolVar(&noColor, "no-color", false, "disable colored output")
}

func initConfig() {
	config.Init(cfgFile)
	if noColor {
		viper.Set("no_color", true)
		_ = os.Setenv("NO_COLOR", "1")
	}
}
