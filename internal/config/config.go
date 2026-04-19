package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Port    int    `json:"port"`
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl"`

	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`

	QBitURL      string `json:"qbitUrl"`
	QBitUser     string `json:"qbitUser"`
	QBitPass     string `json:"qbitPass"`
	QBitCategory string `json:"qbitCategory"`

	SonarrURL    string `json:"sonarrUrl"`
	SonarrAPIKey string `json:"sonarrApiKey"`

	PreferredGroups []string `json:"preferredGroups"`
	PreferredRes    string   `json:"preferredResolution"`

	configPath string
}

func DefaultConfig() *Config {
	return &Config{
		Port:            8978,
		APIKey:          generateAPIKey(),
		BaseURL:         "",
		Username:        "admin",
		PasswordHash:    "",
		QBitURL:         "http://localhost:8081",
		QBitUser:        "",
		QBitPass:        "",
		QBitCategory:    "sonarr-anime",
		SonarrURL:       "http://localhost:8990",
		SonarrAPIKey:    "",
		PreferredGroups: []string{"SubsPlease", "Erai-raws", "EMBER", "Judas", "Tsundere-Raws", "ToonsHub"},
		PreferredRes:    "1080p",
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		configDir := os.Getenv("ANISEARCH_CONFIG")
		if configDir == "" {
			configDir = "/config"
		}
		path = filepath.Join(configDir, "config.json")
	}

	cfg := DefaultConfig()
	cfg.configPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return nil, err
			}
			return cfg, cfg.Save(path)
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.configPath = path

	if v := os.Getenv("ANISEARCH_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Port)
	}
	if v := os.Getenv("ANISEARCH_APIKEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("ANISEARCH_USERNAME"); v != "" {
		cfg.Username = v
	}
	if v := os.Getenv("ANISEARCH_PASSWORD_HASH"); v != "" {
		cfg.PasswordHash = v
	}
	if v := os.Getenv("QBIT_URL"); v != "" {
		cfg.QBitURL = v
	}
	if v := os.Getenv("QBIT_USER"); v != "" {
		cfg.QBitUser = v
	}
	if v := os.Getenv("QBIT_PASS"); v != "" {
		cfg.QBitPass = v
	}
	if v := os.Getenv("SONARR_URL"); v != "" {
		cfg.SonarrURL = v
	}
	if v := os.Getenv("SONARR_APIKEY"); v != "" {
		cfg.SonarrAPIKey = v
	}

	return cfg, nil
}

func (c *Config) GetPath() string {
	return c.configPath
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = c.configPath
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) NeedsSetup() bool {
	return c.PasswordHash == ""
}

func generateAPIKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
