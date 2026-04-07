package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Sync   SyncConfig   `yaml:"sync"`
}

type ServerConfig struct {
	BaseURL         string `yaml:"base_url"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	TodoCollection  string `yaml:"todo_collection"`
	EventCollection string `yaml:"event_collection"`
}

type SyncConfig struct {
	Timezone               string `yaml:"timezone"`
	DueOnlyDurationMinutes int    `yaml:"due_only_duration_minutes"`
	AlarmMinutesBefore     int    `yaml:"alarm_minutes_before"`
	DryRun                 bool   `yaml:"dry_run"`
	PollInterval           string `yaml:"poll_interval"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.BaseURL == "" {
		return fmt.Errorf("server.base_url is required")
	}
	if c.Server.Username == "" {
		return fmt.Errorf("server.username is required")
	}
	if c.Server.Password == "" {
		return fmt.Errorf("server.password is required")
	}
	if c.Server.TodoCollection == "" {
		return fmt.Errorf("server.todo_collection is required")
	}
	if c.Server.EventCollection == "" {
		return fmt.Errorf("server.event_collection is required")
	}
	if c.Sync.Timezone == "" {
		c.Sync.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(c.Sync.Timezone); err != nil {
		return fmt.Errorf("sync.timezone is invalid: %w", err)
	}
	if c.Sync.DueOnlyDurationMinutes <= 0 {
		c.Sync.DueOnlyDurationMinutes = 5
	}
	if c.Sync.AlarmMinutesBefore < 0 {
		return fmt.Errorf("sync.alarm_minutes_before must be >= 0")
	}
	if intervalText := strings.TrimSpace(c.Sync.PollInterval); intervalText != "" {
		interval, err := time.ParseDuration(intervalText)
		if err != nil {
			return fmt.Errorf("sync.poll_interval is invalid: %w", err)
		}
		if interval <= 0 {
			return fmt.Errorf("sync.poll_interval must be > 0")
		}
		c.Sync.PollInterval = interval.String()
	}
	return nil
}

func (s SyncConfig) PollIntervalDuration() time.Duration {
	if strings.TrimSpace(s.PollInterval) == "" {
		return 0
	}

	interval, err := time.ParseDuration(s.PollInterval)
	if err != nil {
		return 0
	}
	return interval
}
