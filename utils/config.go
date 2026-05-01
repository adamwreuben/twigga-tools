package utils

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/adamwreuben/twiggatools/models"
)

func EnsureConfig(cfgFile string) (*models.Config, error) {
	dir := filepath.Dir(cfgFile)
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return &models.Config{}, err
		}
		cfg := &models.Config{
			BaseURL:        "https://twiga.bongocloud.co.tz",
			AccountBaseURL: "https://account.bongocloud.co.tz",
			Status:         false,
			Token:          "--8--aE86fdwidCpbOmEEf9z--Pw",
		}

		SaveConfig(cfgFile, cfg)
		return cfg, nil
	}

	return LoadConfig(cfgFile)

}

func LoadConfig(path string) (*models.Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data models.Config
	json.Unmarshal(b, &data)

	return &data, nil
}

func SaveConfig(path string, cfg *models.Config) error {

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func SaveToken(path string, cfg *models.Config, tok string) error {
	cfg.Token = tok
	cfg.Status = true
	return SaveConfig(path, cfg)
}
