// Package config manages knet's configuration via Viper.
// Config file path: ~/.config/knet/config.yaml
// AI keys also accepted from environment variables:
//   OPENAI_API_KEY, ANTHROPIC_API_KEY, OPENROUTER_API_KEY, KNET_AI_KEY
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Init sets up Viper with config file defaults and environment-variable bindings.
func Init(cfgFile string) {
	viper.SetEnvPrefix("KNET")
	viper.AutomaticEnv()

	// Bind common AI provider env vars
	_ = viper.BindEnv("openai_api_key", "OPENAI_API_KEY")
	_ = viper.BindEnv("anthropic_api_key", "ANTHROPIC_API_KEY")
	_ = viper.BindEnv("openrouter_api_key", "OPENROUTER_API_KEY")
	_ = viper.BindEnv("ai_key", "KNET_AI_KEY")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, _ := os.UserHomeDir()
		viper.AddConfigPath(filepath.Join(home, ".config", "knet"))
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Ignore "file not found" — config is optional
	_ = viper.ReadInConfig()
}

// ConfigDir returns the knet config directory, creating it if necessary.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "knet")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// SnapshotsDir returns the snapshots sub-directory, creating it if necessary.
func SnapshotsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	snap := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snap, 0o700); err != nil {
		return "", err
	}
	return snap, nil
}

// Set writes a key=value pair to the config file.
func Set(key, value string) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	viper.Set(key, value)
	if err := viper.WriteConfigAs(cfgPath); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// GetAIKey returns the API key for the given provider name.
func GetAIKey(provider string) string {
	// Command-line flag takes precedence (already applied by caller)
	switch provider {
	case "openai":
		if k := viper.GetString("openai_api_key"); k != "" {
			return k
		}
	case "anthropic":
		if k := viper.GetString("anthropic_api_key"); k != "" {
			return k
		}
	case "openrouter":
		if k := viper.GetString("openrouter_api_key"); k != "" {
			return k
		}
	}
	return viper.GetString("ai_key")
}
