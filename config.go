package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Manifest ManifestInfo `toml:"manifest"`
}

type ManifestInfo struct {
	Path   string `toml:"path"`
	File   string `toml:"file"`
	Branch string `toml:"branch"`
}

func SaveConfig(cfg *Config) error {
	confFile := filepath.Join(ConfDir, "config")
	f, err := os.Create(confFile)
	if err != nil {
		return fmt.Errorf("Fail to create file: %s", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	err = encoder.Encode(cfg)
	if err != nil {
		return fmt.Errorf("Fail to marshal: %s", err)
	}

	return nil
}

func LoadConfig() (*Config, error) {
	confFile := filepath.Join(ConfDir, "config")
	f, err := os.Open(confFile)
	if err != nil {
		return nil, fmt.Errorf("Fail to open file: %s", err)
	}
	defer f.Close()

	decoder := toml.NewDecoder(f)
	var cfg Config
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("Fail to unmarshal: %s", err)
	}

	return &cfg, nil
}
