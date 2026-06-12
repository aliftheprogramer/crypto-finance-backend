package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type SourceConfig struct {
	Type      string   `yaml:"type"`
	Name      string   `yaml:"name"`
	APIKey    string   `yaml:"api_key"`
	APISecret string   `yaml:"api_secret"`
	Address   string   `yaml:"address"`
	Chains    []string `yaml:"chains"`
}

type SourcesFile struct {
	Sources []SourceConfig `yaml:"sources"`
}

type Config struct {
	Port              string
	DeepSeekAPIKey    string
	CryptoPanicAPIKey string
	Sources           []SourceConfig
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using defaults")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	sources := loadSources()

	cfg := &Config{
		Port:              port,
		DeepSeekAPIKey:    os.Getenv("DEEPSEEK_API_KEY"),
		CryptoPanicAPIKey: os.Getenv("CRYPTOPANIC_API_KEY"),
		Sources:           sources,
	}

	if len(cfg.Sources) == 0 {
		log.Println("No sources configured — using mock source")
		cfg.Sources = []SourceConfig{
			{Type: "mock", Name: "Mock"},
		}
	}

	return cfg
}

func loadSources() []SourceConfig {
	data, err := os.ReadFile("sources.yaml")
	if err != nil {
		return nil
	}

	var sf SourcesFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		log.Printf("Warning: failed to parse sources.yaml: %v", err)
		return nil
	}

	var valid []SourceConfig
	for _, s := range sf.Sources {
		if s.Type == "" || s.Name == "" {
			log.Println("Warning: skipping source with empty type or name")
			continue
		}
		valid = append(valid, s)
	}

	return valid
}
