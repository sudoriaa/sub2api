package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type questionKeysFake struct {
	keys map[int64]*APIKey
	err  error
}

func (k *questionKeysFake) GetByID(_ context.Context, id int64) (*APIKey, error) {
	if k.err != nil {
		return nil, k.err
	}
	if key := k.keys[id]; key != nil {
		return key, nil
	}
	return nil, ErrAPIKeyNotFound
}
func (k *questionKeysFake) SearchAPIKeys(_ context.Context, _ int64, q string, _ int) ([]APIKey, error) {
	if k.err != nil {
		return nil, k.err
	}
	keys := []APIKey{}
	for _, key := range k.keys {
		if strings.Contains(key.Name, q) {
			keys = append(keys, *key)
		}
	}
	return keys, nil
}
func questionInput() *PelicanQuestionGeneratorConfig {
	return &PelicanQuestionGeneratorConfig{APIKeyID: 3, ModelID: "generator-model", Prompt: DefaultPelicanQuestionGeneratorPrompt}
}
func questionConfig() *PelicanTestConfig {
	return &PelicanTestConfig{Prompt: "legacy", QuestionGenerator: questionInput(), FallbackPrompts: []string{"fallback A", "fallback B", "fallback C"}}
}

func TestPelicanQuestionGeneratorPlanValidation(t *testing.T) {
	svc, _ := newGroupTestService(newGroupTestRepoFake(), &groupTestRouterFake{}, nil)
	svc.questionGenerator = &pelicanQuestionGenerator{keys: &questionKeysFake{keys: map[int64]*APIKey{3: {ID: 3, Key: "synthetic-secret", Status: StatusActive}}}}
	input := PelicanGroupTestPlanInput{GroupID: 4, ModelID: "tested-model", ReasoningEffort: "high", QuestionGenerator: &PelicanQuestionGeneratorConfig{APIKeyID: 3, ModelID: " generator-model "}, FallbackPrompts: []string{" first ", "second"}}
	plan, err := svc.CreatePlan(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, DefaultPelicanQuestionGeneratorPrompt, plan.PelicanConfig.QuestionGenerator.Prompt)
	require.Equal(t, "generator-model", plan.PelicanConfig.QuestionGenerator.ModelID)
	require.Equal(t, []string{"first", "second"}, plan.PelicanConfig.FallbackPrompts)
	require.Empty(t, input.QuestionGenerator.Prompt, "validation must not mutate the caller")
	require.Empty(t, plan.PelicanConfig.QuestionSource)
	for _, bad := range []*PelicanQuestionGeneratorConfig{{APIKeyID: 0, ModelID: "m"}, {APIKeyID: 3}, {APIKeyID: 404, ModelID: "m"}, {APIKeyID: 3, ModelID: strings.Repeat("m", 101)}, {APIKeyID: 3, ModelID: "m", Prompt: strings.Repeat("p", 32001)}} {
		invalid := input
		invalid.QuestionGenerator = bad
		_, err = svc.CreatePlan(context.Background(), invalid)
		require.Error(t, err)
	}
	for _, pool := range [][]string{{}, {""}, {"ok", " "}, {strings.Repeat("字", 10667)}, strings.Split(strings.Repeat("q,", 21), ","), {strings.Repeat("x", 32000), strings.Repeat("x", 32000), strings.Repeat("x", 32000), strings.Repeat("x", 32000), "x"}} {
		invalid := input
		invalid.FallbackPrompts = pool
		_, err = svc.CreatePlan(context.Background(), invalid)
		require.Error(t, err)
	}
	// An edit can keep a revoked key and update fallbacks; runtime does not bypass revocation.
	svc.questionGenerator.keys = &questionKeysFake{}
	input.QuestionGenerator = plan.PelicanConfig.QuestionGenerator
	_, err = svc.UpdatePlan(context.Background(), plan.ID, input)
	require.NoError(t, err)
	input.QuestionGenerator = nil
	updated, err := svc.UpdatePlan(context.Background(), plan.ID, input)
	require.NoError(t, err)
	require.Nil(t, updated.PelicanConfig.QuestionGenerator)
}

func TestPelicanQuestionGeneratorKeyOptionsNeverExposeCredentials(t *testing.T) {
	svc := &PelicanGroupTestService{questionGenerator: &pelicanQuestionGenerator{keys: &questionKeysFake{keys: map[int64]*APIKey{
		3: {ID: 3, Name: "chosen", UserID: 7, Key: "synthetic-secret-a", Status: StatusActive},
		4: {ID: 4, Name: "other", UserID: 8, Key: "synthetic-secret-b", Status: StatusAPIKeyDisabled},
	}}}}
	keys, err := svc.ListQuestionGeneratorKeys(context.Background(), "other", 3)
	require.NoError(t, err)
	require.Len(t, keys, 2)
	data, err := json.Marshal(keys)
	require.NoError(t, err)
	require.NotContains(t, string(data), "synthetic-secret")
	require.NotContains(t, string(data), `"key":`)
	require.Contains(t, string(data), `"available":false`)
	_, err = svc.ListQuestionGeneratorKeys(context.Background(), strings.Repeat("q", 201), 0)
	require.Error(t, err)
}

func TestPelicanQuestionGeneratorOnlyFallsBackOnGenerationFailure(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		err          error
		source       string
	}{
		{"success", "创建一个 HTML，内容是 SVG 绘制一只猫打篮球的 2D 动画，你不需要任何测试", nil, "ai"},
		{"empty", "  ", nil, "fallback"}, {"actual HTML", "<html></html>", nil, "fallback"},
		{"code fence", "```html\n<html></html>\n```", nil, "fallback"},
		{"too large", strings.Repeat("x", 32001), nil, "fallback"},
		{"error body", "", errors.New("leaked synthetic-secret in raw provider error"), "fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			svc := &PelicanGroupTestService{questionGenerator: &pelicanQuestionGenerator{request: func(_ context.Context, in *PelicanQuestionGeneratorConfig) (string, error) {
				calls++
				require.Equal(t, int64(3), in.APIKeyID)
				return tc.output, tc.err
			}}}
			original := questionConfig()
			result, err := svc.prepareGroupQuestion(context.Background(), original)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, tc.source, result.QuestionSource)
			if tc.source == "ai" {
				require.Equal(t, tc.output, result.Prompt)
				require.Empty(t, result.QuestionGenerationError)
			} else {
				require.Contains(t, original.FallbackPrompts, result.Prompt)
				require.NotEmpty(t, result.QuestionGenerationError)
				require.NotContains(t, result.QuestionGenerationError, "synthetic-secret")
			}
			require.Equal(t, "legacy", original.Prompt)
			require.Empty(t, original.QuestionSource)
			result.FallbackPrompts[0] = "edited result"
			result.QuestionGenerator.Prompt = "edited result"
			require.Equal(t, "fallback A", original.FallbackPrompts[0])
			require.Equal(t, DefaultPelicanQuestionGeneratorPrompt, original.QuestionGenerator.Prompt)
		})
	}
	svc := &PelicanGroupTestService{}
	result, err := svc.prepareGroupQuestion(context.Background(), questionConfig())
	require.NoError(t, err)
	require.Equal(t, "generator_unavailable", result.QuestionGenerationError)
	legacy := &PelicanTestConfig{Prompt: "fixed"}
	result, err = svc.prepareGroupQuestion(context.Background(), legacy)
	require.NoError(t, err)
	require.Equal(t, "fixed", result.Prompt)
	require.Equal(t, intelligenceTestPrompt(result), result.ActualPrompt)
	require.Empty(t, legacy.ActualPrompt, "execution evidence must not mutate the saved plan")
}

func TestPelicanQuestionGeneratorPreservesFrozenRandomBank(t *testing.T) {
	calls := 0
	svc := &PelicanGroupTestService{questionGenerator: &pelicanQuestionGenerator{request: func(context.Context, *PelicanQuestionGeneratorConfig) (string, error) {
		calls++
		return "must not replace the shared bank question", nil
	}}}
	config := questionConfig()
	config.PromptMode = PelicanPromptModeRandom
	frozen := freezePelicanQuestion(config, "scheduled:2026-11-12T00:00:00Z", nil)
	for i := 0; i < 3; i++ {
		result, err := svc.prepareGroupQuestion(context.Background(), frozen)
		require.NoError(t, err)
		require.Equal(t, frozen.Prompt, result.Prompt)
		require.Equal(t, frozen.PelicanQuestion, result.PelicanQuestion)
		require.Equal(t, frozen.ActualPrompt, result.ActualPrompt)
	}
	require.Zero(t, calls)
	require.Equal(t, "legacy", config.Prompt)
}

func TestPelicanQuestionGeneratorRefreshesActualPromptEvidence(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		svc := &PelicanGroupTestService{questionGenerator: &pelicanQuestionGenerator{request: func(context.Context, *PelicanQuestionGeneratorConfig) (string, error) {
			if fallback {
				return "", errors.New("synthetic upstream unavailable")
			}
			return "generated scene", nil
		}}}
		frozen := freezePelicanQuestion(questionConfig(), "manual-group:fixture", nil)
		result, err := svc.prepareGroupQuestion(context.Background(), frozen)
		require.NoError(t, err)
		require.Equal(t, intelligenceTestPrompt(result), result.ActualPrompt)
		require.NotEqual(t, frozen.ActualPrompt, result.ActualPrompt)
		require.Nil(t, result.PelicanQuestion, "custom questions must not carry a built-in rubric")
	}
}

func TestPelicanQuestionGeneratorKeepsQuestionAcrossAccountRetries(t *testing.T) {
	plan := groupTestPlan(1)
	plan.PelicanConfig.QuestionGenerator = questionInput()
	plan.PelicanConfig.FallbackPrompts = []string{"fallback"}
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "first")}, {account: account(12, "second")}}}
	questions := 0
	svc, _ := newGroupTestService(newGroupTestRepoFake(plan), router, func(_ context.Context, id int64, _ string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
		require.Equal(t, "generated question", cfg.Prompt)
		if id == 11 {
			return &ScheduledTestResult{Status: "failed", ErrorMessage: "API returned 503"}, nil
		}
		return &ScheduledTestResult{Status: "success", ResponseText: "<svg></svg>"}, nil
	})
	svc.questionGenerator = &pelicanQuestionGenerator{request: func(context.Context, *PelicanQuestionGeneratorConfig) (string, error) {
		questions++
		return "generated question", nil
	}}
	result := svc.runSamples(context.Background(), plan)[0]
	require.Equal(t, "success", result.Status)
	require.Equal(t, 1, questions)
	require.Len(t, result.Attempts, 1)
	require.Equal(t, "generated question", result.PelicanConfig.Prompt)
	require.Equal(t, "ai", result.PelicanConfig.QuestionSource)
	require.Equal(t, "draw", plan.PelicanConfig.Prompt)
}

func TestPelicanQuestionGeneratorParallelSamplesAreIndependent(t *testing.T) {
	plan := groupTestPlan(3)
	plan.PelicanConfig.QuestionGenerator = questionInput()
	plan.PelicanConfig.FallbackPrompts = []string{"A", "B"}
	router := &groupTestRouterFake{steps: []routeStep{{account: account(11, "a")}, {account: account(12, "b")}, {account: account(13, "c")}}}
	var calls atomic.Int64
	var mu sync.Mutex
	seen := map[string]bool{}
	svc, _ := newGroupTestService(newGroupTestRepoFake(plan), router, func(_ context.Context, _ int64, _ string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
		mu.Lock()
		seen[cfg.Prompt] = true
		mu.Unlock()
		return &ScheduledTestResult{Status: "success", ResponseText: "<svg></svg>"}, nil
	})
	svc.questionGenerator = &pelicanQuestionGenerator{request: func(context.Context, *PelicanQuestionGeneratorConfig) (string, error) {
		return fmt.Sprintf("question %d", calls.Add(1)), nil
	}}
	results := svc.runSamples(context.Background(), plan)
	require.Len(t, results, 3)
	require.Len(t, seen, 3)
	require.EqualValues(t, 3, calls.Load())
	for _, result := range results {
		require.Equal(t, "success", result.Status)
		require.Equal(t, "ai", result.PelicanConfig.QuestionSource)
	}
	require.Equal(t, "draw", plan.PelicanConfig.Prompt)
}

func TestPelicanQuestionGeneratorCancellationDoesNotStartFallbackAnswer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := &PelicanGroupTestService{questionGenerator: &pelicanQuestionGenerator{request: func(ctx context.Context, _ *PelicanQuestionGeneratorConfig) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}}}
	_, err := svc.prepareGroupQuestion(ctx, questionConfig())
	require.EqualError(t, err, "question_generation_cancelled")
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = svc.generateGroupQuestion(ctx, questionInput())
	require.EqualError(t, err, "generator_timeout")
}

func TestPelicanQuestionGeneratorHTTPUsesNormalStreamingKeyRequest(t *testing.T) {
	keys := &questionKeysFake{keys: map[int64]*APIKey{3: {ID: 3, Key: "synthetic-site-key", Status: StatusActive}}}
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "Bearer synthetic-site-key", r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, true, body["stream"])
		require.Equal(t, "generator-model", body["model"])
		require.Equal(t, DefaultPelicanQuestionGeneratorPrompt, body["messages"].([]any)[0].(map[string]any)["content"])
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"创建一个 HTML，\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"内容是 SVG 绘制皮卡丘游泳的 2D 动画，你不需要任何测试\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	output, err := requestPelicanQuestion(context.Background(), keys, server.Client(), server.URL, questionInput())
	require.NoError(t, err)
	require.Contains(t, output, "皮卡丘游泳")
	require.EqualValues(t, 1, calls.Load())
	keys.keys[3].Status = StatusAPIKeyDisabled
	_, err = requestPelicanQuestion(context.Background(), keys, server.Client(), server.URL, questionInput())
	require.EqualError(t, err, "generator_key_unavailable")
	require.EqualValues(t, 1, calls.Load())
	configured := newPelicanQuestionGenerator(nil, &config.Config{Server: config.ServerConfig{Port: 8080}})
	require.NotNil(t, configured.request)
}

func TestPelicanQuestionGeneratorStreamRejectsIncompleteRefusedOrOversized(t *testing.T) {
	for _, body := range []string{
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n",
		"data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n",
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"refusal\":\"no\"}}]}\n\n",
		"data: {\"error\":{\"message\":\"secret\"}}\n\n",
		"data: invalid JSON\n\n",
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"" + strings.Repeat("x", 32001) + "\"}}]}\n\n",
		strings.Repeat(": heartbeat\n", 30000),
	} {
		_, err := readPelicanQuestionStream(&io.LimitedReader{R: strings.NewReader(body), N: pelicanQuestionMaxWireBytes + 1})
		require.Error(t, err)
	}
}

func TestPelicanQuestionGeneratorHTTP429IsNotRetriedOrEchoed(t *testing.T) {
	keys := &questionKeysFake{keys: map[int64]*APIKey{3: {ID: 3, Key: "synthetic-key", Status: StatusActive}}}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":"synthetic-key raw body"}`)
	}))
	defer server.Close()
	_, err := requestPelicanQuestion(context.Background(), keys, server.Client(), server.URL, questionInput())
	require.EqualError(t, err, "generator_http_429")
	require.Equal(t, 1, calls)
	require.NotContains(t, err.Error(), "synthetic-key")
}
