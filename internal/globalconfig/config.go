package globalconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	GitHubUser     string `json:"githubUser"`
	RepositoryName string `json:"repositoryName"`
	PackageManager string `json:"packageManager"`
	Notify         bool   `json:"notify"`
}

func getConfigFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".wfkit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

func LoadConfig() (*Config, error) {
	file, err := getConfigFile()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				PackageManager: "bun",
			}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	file, err := getConfigFile()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(file, data, 0644)
}
