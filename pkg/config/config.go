package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Streams []string   `yaml:"streams"`
	Sink    SinkConfig `yaml:"sink"`
}

type SinkConfig struct {
	Type string      `yaml:"type"` // "console", "file", "http"
	File *FileConfig `yaml:"file,omitempty"`
	Http *HttpConfig `yaml:"http,omitempty"`
}

type FileConfig struct {
	Path string `yaml:"path"`
}

type HttpConfig struct {
	URL         string `yaml:"url"`
	Method      string `yaml:"method"`
	ContentType string `yaml:"content_type"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
