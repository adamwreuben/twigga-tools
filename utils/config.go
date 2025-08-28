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
			Token:          "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJkYXRhIjp7ImFwcElkIjoidHdpZ2dhdG9vbHMiLCJhcHBTZWNyZXQiOiI1MjAxYzhmNC1hYjE3LTRmZTQtOTcxZC1lZGMwMzgzOTMwZGMiLCJleHAiOjE3NTg1Mzg1NTR9LCJleHAiOjE3NTg1Mzg1NTR9.wSJM1YnC4VdOGzSUmZ3r8v0uOJGA7g9L2X3fgQkdt6ciafX9SLnVK8zkExjC5arrutD4tRolyeUg-YpTJaJS4mOdxL3LMX8uulnbGUhpEbrawFMyGuStsZ7dgLxFpUxlAHbaQfutRFnoPYZnsjqmhWsgeW44taDe0S7HaypNqJJsNXK21iA-8-bToFKepTbLeKl9jCLfseyyGfrFcuQBXjuhjnJiwfQXFkKeoZ8-aE86fdwidCpbOmEEf9z-XwDwo_QzzbTyQh7Npr0MQOggXlVWF7TRhDqQa4X0EH4_ErmIGZEC9W57gvKiShdZYrhl2VYtgwHP1bd7UeWr6cw-Pw",
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
