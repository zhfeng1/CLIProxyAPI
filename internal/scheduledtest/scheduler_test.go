package scheduledtest

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestBuildTasks_UsesConfiguredModelAndProvider(t *testing.T) {
	cfg := &config.Config{
		OpenAICompatibility: []config.OpenAICompatibility{{
			Name:    "OpenRouter",
			BaseURL: "https://openrouter.ai/api/v1",
			Models:  []config.OpenAICompatibilityModel{{Name: "upstream", Alias: "alias"}},
			ScheduledTest: &config.ProviderScheduledTest{
				Enabled:    true,
				Model:      "alias",
				Cron:       "*/5 * * * *",
				MaxResults: 7,
			},
		}},
	}

	tasks := buildTasks(cfg)
	if len(tasks) != 1 {
		t.Fatalf("tasks length = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.provider != "openrouter" {
		t.Fatalf("provider = %q, want %q", task.provider, "openrouter")
	}
	if task.displayName != "OpenRouter" {
		t.Fatalf("displayName = %q, want %q", task.displayName, "OpenRouter")
	}
	if task.model != "alias" {
		t.Fatalf("model = %q, want %q", task.model, "alias")
	}
	if task.maxResults != 7 {
		t.Fatalf("maxResults = %d, want 7", task.maxResults)
	}
}

func TestBuildTasks_DefaultsModelAndSkipsInvalidCron(t *testing.T) {
	cfg := &config.Config{
		OpenAICompatibility: []config.OpenAICompatibility{
			{
				Name:    "valid",
				BaseURL: "https://example.com/v1",
				Models:  []config.OpenAICompatibilityModel{{Name: "upstream", Alias: "alias"}},
				ScheduledTest: &config.ProviderScheduledTest{
					Enabled: true,
					Cron:    "0 * * * *",
				},
			},
			{
				Name:    "invalid",
				BaseURL: "https://example.com/v1",
				Models:  []config.OpenAICompatibilityModel{{Name: "bad"}},
				ScheduledTest: &config.ProviderScheduledTest{
					Enabled: true,
					Model:   "bad",
					Cron:    "invalid",
				},
			},
		},
	}

	tasks := buildTasks(cfg)
	if len(tasks) != 1 {
		t.Fatalf("tasks length = %d, want 1", len(tasks))
	}
	if tasks[0].model != "alias" {
		t.Fatalf("default model = %q, want %q", tasks[0].model, "alias")
	}
}
