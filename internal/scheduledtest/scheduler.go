package scheduledtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	log "github.com/sirupsen/logrus"
)

const defaultMaxResults = 20

type task struct {
	key         string
	provider    string
	displayName string
	model       string
	cron        string
	spec        CronSpec
	maxResults  int
}

// Result captures one scheduled provider test result.
type Result struct {
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Cron          string    `json:"cron"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	DurationMS    int64     `json:"duration_ms"`
	Success       bool      `json:"success"`
	StatusCode    int       `json:"status_code,omitempty"`
	Error         string    `json:"error,omitempty"`
	ResponseBytes int       `json:"response_bytes,omitempty"`
	AuthID        string    `json:"auth_id,omitempty"`
}

// Scheduler runs configured OpenAI-compatible provider health checks.
type Scheduler struct {
	mu      sync.Mutex
	manager *coreauth.Manager
	tasks   []task
	results map[string][]Result
	lastRun map[string]time.Time
	running map[string]struct{}
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewScheduler creates a scheduler and loads the initial configuration.
func NewScheduler(cfg *config.Config, manager *coreauth.Manager) *Scheduler {
	s := &Scheduler{
		manager: manager,
		results: make(map[string][]Result),
		lastRun: make(map[string]time.Time),
		running: make(map[string]struct{}),
	}
	s.Update(cfg, manager)
	return s
}

// ConfigHasEnabled reports whether cfg contains at least one enabled scheduled test.
func ConfigHasEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	for i := range cfg.OpenAICompatibility {
		entry := cfg.OpenAICompatibility[i]
		if entry.Disabled || entry.ScheduledTest == nil || !entry.ScheduledTest.Enabled {
			continue
		}
		return true
	}
	return false
}

// Update replaces the scheduler task set from cfg.
func (s *Scheduler) Update(cfg *config.Config, manager *coreauth.Manager) {
	if s == nil {
		return
	}
	tasks := buildTasks(cfg)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.manager = manager
	s.tasks = tasks
}

// Start launches the background loop. It is safe to call multiple times.
func (s *Scheduler) Start(ctx context.Context) {
	if s == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	s.mu.Unlock()

	go func() {
		defer close(done)
		s.loop(runCtx)
	}()
}

// Stop stops the background loop.
func (s *Scheduler) Stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// Results returns a copy of retained scheduled test results grouped by provider.
func (s *Scheduler) Results() map[string][]Result {
	if s == nil {
		return map[string][]Result{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]Result, len(s.results))
	for provider, results := range s.results {
		out[provider] = append([]Result(nil), results...)
	}
	return out
}

func (s *Scheduler) loop(ctx context.Context) {
	for {
		now := time.Now()
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		timer := time.NewTimer(time.Until(nextMinute))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case tickTime := <-timer.C:
			s.runDue(tickTime)
		}
	}
}

func (s *Scheduler) runDue(now time.Time) {
	now = now.Truncate(time.Minute)

	s.mu.Lock()
	manager := s.manager
	tasks := append([]task(nil), s.tasks...)
	runnable := make([]task, 0, len(tasks))
	for _, task := range tasks {
		if !task.spec.Match(now) {
			continue
		}
		if last, ok := s.lastRun[task.key]; ok && last.Equal(now) {
			continue
		}
		if _, ok := s.running[task.key]; ok {
			log.WithFields(log.Fields{
				"provider": task.displayName,
				"model":    task.model,
			}).Debug("scheduled provider test skipped because previous run is still active")
			continue
		}
		s.lastRun[task.key] = now
		s.running[task.key] = struct{}{}
		runnable = append(runnable, task)
	}
	s.mu.Unlock()

	if manager == nil {
		for _, task := range runnable {
			s.finishTask(task.key)
		}
		return
	}
	for _, task := range runnable {
		go s.runTask(task, manager)
	}
}

func (s *Scheduler) runTask(task task, manager *coreauth.Manager) {
	defer s.finishTask(task.key)

	started := time.Now()
	metadata := map[string]any{
		coreexecutor.RequestedModelMetadataKey: task.model,
		coreexecutor.RequestPathMetadataKey:    "/v1/chat/completions",
	}
	payload, _ := json.Marshal(map[string]any{
		"model": task.model,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 1,
		"stream":     false,
	})

	resp, err := manager.Execute(context.Background(), []string{task.provider}, coreexecutor.Request{
		Model:   task.model,
		Payload: payload,
	}, coreexecutor.Options{
		OriginalRequest: payload,
		SourceFormat:    sdktranslator.FromString("openai"),
		Metadata:        metadata,
	})

	finished := time.Now()
	result := Result{
		Provider:      task.displayName,
		Model:         task.model,
		Cron:          task.cron,
		StartedAt:     started,
		FinishedAt:    finished,
		DurationMS:    finished.Sub(started).Milliseconds(),
		Success:       err == nil,
		ResponseBytes: len(resp.Payload),
	}
	if authID, _ := metadata[coreexecutor.SelectedAuthMetadataKey].(string); strings.TrimSpace(authID) != "" {
		result.AuthID = strings.TrimSpace(authID)
	}
	if err != nil {
		result.Error = truncateError(err.Error())
		if statusErr, ok := err.(interface{ StatusCode() int }); ok {
			result.StatusCode = statusErr.StatusCode()
		}
		log.WithFields(log.Fields{
			"provider": task.displayName,
			"model":    task.model,
			"error":    result.Error,
		}).Warn("scheduled provider test failed")
	} else {
		result.StatusCode = 200
		log.WithFields(log.Fields{
			"provider": task.displayName,
			"model":    task.model,
			"duration": result.DurationMS,
		}).Info("scheduled provider test succeeded")
	}

	s.appendResult(task, result)
}

func (s *Scheduler) finishTask(key string) {
	s.mu.Lock()
	delete(s.running, key)
	s.mu.Unlock()
}

func (s *Scheduler) appendResult(task task, result Result) {
	maxResults := task.maxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	results := append([]Result{result}, s.results[task.displayName]...)
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	s.results[task.displayName] = results
}

func buildTasks(cfg *config.Config) []task {
	if cfg == nil {
		return nil
	}
	tasks := make([]task, 0, len(cfg.OpenAICompatibility))
	for i := range cfg.OpenAICompatibility {
		entry := cfg.OpenAICompatibility[i]
		if entry.Disabled || entry.ScheduledTest == nil || !entry.ScheduledTest.Enabled {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(entry.Name))
		if provider == "" {
			provider = "openai-compatibility"
		}
		displayName := strings.TrimSpace(entry.Name)
		if displayName == "" {
			displayName = provider
		}
		model := strings.TrimSpace(entry.ScheduledTest.Model)
		if model == "" {
			model = firstOpenAICompatModel(entry.Models)
		}
		if model == "" {
			log.WithField("provider", displayName).Warn("scheduled provider test skipped: model is empty")
			continue
		}
		cronExpr := strings.Join(strings.Fields(entry.ScheduledTest.Cron), " ")
		spec, err := ParseCron(cronExpr)
		if err != nil {
			log.WithFields(log.Fields{
				"provider": displayName,
				"cron":     cronExpr,
				"error":    err,
			}).Warn("scheduled provider test skipped: invalid cron")
			continue
		}
		tasks = append(tasks, task{
			key:         fmt.Sprintf("%d:%s", i, provider),
			provider:    provider,
			displayName: displayName,
			model:       model,
			cron:        cronExpr,
			spec:        spec,
			maxResults:  entry.ScheduledTest.MaxResults,
		})
	}
	return tasks
}

func firstOpenAICompatModel(models []config.OpenAICompatibilityModel) string {
	for _, model := range models {
		if alias := strings.TrimSpace(model.Alias); alias != "" {
			return alias
		}
		if name := strings.TrimSpace(model.Name); name != "" {
			return name
		}
	}
	return ""
}

func truncateError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 1000 {
		return message
	}
	return message[:1000]
}
