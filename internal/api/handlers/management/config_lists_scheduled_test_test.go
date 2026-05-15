package management

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestPatchOpenAICompatScheduledTest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			OpenAICompatibility: []config.OpenAICompatibility{{
				Name:    "openrouter",
				BaseURL: "https://openrouter.ai/api/v1",
			}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/openai-compatibility", strings.NewReader(`{
		"name":"openrouter",
		"value":{
			"scheduled-test":{
				"enabled":true,
				"model":"kimi-k2",
				"cron":"*/15 * * * *",
				"max-results":5
			}
		}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchOpenAICompat(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := h.cfg.OpenAICompatibility[0].ScheduledTest
	if got == nil || !got.Enabled {
		t.Fatalf("scheduled-test was not enabled: %#v", got)
	}
	if got.Model != "kimi-k2" {
		t.Fatalf("model = %q, want %q", got.Model, "kimi-k2")
	}
	if got.Cron != "*/15 * * * *" {
		t.Fatalf("cron = %q, want %q", got.Cron, "*/15 * * * *")
	}
	if got.MaxResults != 5 {
		t.Fatalf("max-results = %d, want 5", got.MaxResults)
	}
}

func TestPatchOpenAICompatScheduledTestRejectsInvalidCron(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			OpenAICompatibility: []config.OpenAICompatibility{{
				Name:    "openrouter",
				BaseURL: "https://openrouter.ai/api/v1",
			}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/openai-compatibility", strings.NewReader(`{
		"name":"openrouter",
		"value":{
			"scheduled-test":{
				"enabled":true,
				"model":"kimi-k2",
				"cron":"60 * * * *"
			}
		}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchOpenAICompat(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if h.cfg.OpenAICompatibility[0].ScheduledTest != nil {
		t.Fatalf("scheduled-test should not be saved on invalid cron")
	}
}

func TestPatchClaudeKeyScheduledTest(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			ClaudeKey: []config.ClaudeKey{{
				APIKey: "sk-ant-test",
			}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/claude-api-key", strings.NewReader(`{
		"index":0,
		"value":{
			"scheduled-test":{
				"enabled":true,
				"model":"claude-sonnet-latest",
				"cron":"*/20 * * * *",
				"max-results":4
			}
		}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchClaudeKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := h.cfg.ClaudeKey[0].ScheduledTest
	if got == nil || !got.Enabled {
		t.Fatalf("scheduled-test was not enabled: %#v", got)
	}
	if got.Model != "claude-sonnet-latest" {
		t.Fatalf("model = %q, want %q", got.Model, "claude-sonnet-latest")
	}
	if got.Cron != "*/20 * * * *" {
		t.Fatalf("cron = %q, want %q", got.Cron, "*/20 * * * *")
	}
	if got.MaxResults != 4 {
		t.Fatalf("max-results = %d, want 4", got.MaxResults)
	}
}

func TestPatchClaudeKeyScheduledTestRejectsInvalidCron(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			ClaudeKey: []config.ClaudeKey{{
				APIKey: "sk-ant-test",
			}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/claude-api-key", strings.NewReader(`{
		"index":0,
		"value":{
			"scheduled-test":{
				"enabled":true,
				"model":"claude-sonnet-latest",
				"cron":"*/0 * * * *"
			}
		}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchClaudeKey(c)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if h.cfg.ClaudeKey[0].ScheduledTest != nil {
		t.Fatalf("scheduled-test should not be saved on invalid cron")
	}
}
