package config

import "testing"

func TestConfigValidateSupportsSplitSourceAndTarget(t *testing.T) {
	cfg := Config{
		Source: SourceConfig{
			BaseURL:        "https://vikunja.example.com",
			Username:       "source-user",
			Password:       "source-pass",
			TodoCollection: "/dav/projects/12",
		},
		Target: TargetConfig{
			BaseURL:         "https://radicale.example.com",
			Username:        "target-user",
			Password:        "target-pass",
			EventCollection: "/target/task-reminders/",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate split config: %v", err)
	}

	if cfg.Source.Type != "caldav" {
		t.Fatalf("expected default source type caldav, got %q", cfg.Source.Type)
	}
	if cfg.Target.Type != "caldav" {
		t.Fatalf("expected default target type caldav, got %q", cfg.Target.Type)
	}
	if cfg.Sync.Timezone != "UTC" {
		t.Fatalf("expected default timezone UTC, got %q", cfg.Sync.Timezone)
	}
	if cfg.Sync.DueOnlyDurationMinutes != 5 {
		t.Fatalf("expected default due-only duration 5, got %d", cfg.Sync.DueOnlyDurationMinutes)
	}
}
