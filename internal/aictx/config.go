package aictx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultConfigPath = ".ai-context/config.json"

func defaultConfig() Config {
	return Config{
		Version: 1,
		IDL: IDLConfig{
			Root:    "../idl",
			Include: []string{"**/*.thrift"},
		},
		Knowledge: KnowledgeConfig{Root: ".ai-context"},
	}
}

func loadConfig(configPath string) (Config, string, error) {
	if configPath == "" {
		configPath = defaultConfigPath
	}
	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		return Config{}, "", err
	}
	data, err := os.ReadFile(absConfig)
	if err != nil {
		return Config{}, "", fmt.Errorf("read config %s: %w", absConfig, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, "", fmt.Errorf("parse config %s: %w", absConfig, err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.IDL.Root == "" {
		return Config{}, "", errors.New("config idl.root is required")
	}
	if len(cfg.IDL.Include) == 0 {
		cfg.IDL.Include = []string{"**/*.thrift"}
	}
	if cfg.Knowledge.Root == "" {
		cfg.Knowledge.Root = ".ai-context"
	}
	repoRoot := filepath.Dir(filepath.Dir(absConfig))
	if filepath.Base(filepath.Dir(absConfig)) != ".ai-context" {
		repoRoot = filepath.Dir(absConfig)
	}
	return cfg, repoRoot, nil
}

func resolvePath(base, p string) string {
	if p == "" {
		return base
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}

func writeJSONFile(path string, v any, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
