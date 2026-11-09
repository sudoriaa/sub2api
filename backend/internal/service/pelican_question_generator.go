package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const DefaultPelicanQuestionGeneratorPrompt = "帮我生成个提示词，关于动物、昆虫、常见动漫角色做某个动作、运动的场景，你的输出必须严格按照这个示例的格式输出：创建一个 HTML，内容是 SVG 绘制一个鹈鹕骑自行车的 2D 动画，你不需要任何测试"

const (
	pelicanQuestionMaxFallbacks = 20
	pelicanQuestionMaxBytes     = 32000
	pelicanQuestionMaxWireBytes = 256 * 1024
	pelicanQuestionTimeout      = 90 * time.Second
)

// Only an identifier is saved in plan/result JSON. The key is resolved afresh
// for each request, so revocation, rotation and ordinary gateway policy apply.
type PelicanQuestionGeneratorConfig struct {
	APIKeyID int64  `json:"api_key_id"`
	ModelID  string `json:"model_id"`
	Prompt   string `json:"prompt"`
}

type PelicanQuestionGeneratorKey struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	UserID    int64  `json:"user_id"`
	Status    string `json:"status"`
	Available bool   `json:"available"`
}

type pelicanQuestionKeys interface {
	GetByID(context.Context, int64) (*APIKey, error)
	SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error)
}

type pelicanQuestionGenerator struct {
	keys    pelicanQuestionKeys
	request func(context.Context, *PelicanQuestionGeneratorConfig) (string, error)
}

func newPelicanQuestionGenerator(keys APIKeyRepository, cfg *config.Config) *pelicanQuestionGenerator {
	g := &pelicanQuestionGenerator{keys: keys}
	if cfg != nil && cfg.Server.Port > 0 {
		// Never derive a destination from public site URLs, model output or an
		// operator-provided URL. Do not use environment proxy configuration.
		endpoint := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Server.Port)) + "/v1/chat/completions"
		client := &http.Client{Transport: &http.Transport{Proxy: nil},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		g.request = func(ctx context.Context, input *PelicanQuestionGeneratorConfig) (string, error) {
			return requestPelicanQuestion(ctx, keys, client, endpoint, input)
		}
	}
	return g
}

func (s *PelicanGroupTestService) ListQuestionGeneratorKeys(ctx context.Context, query string, selectedID int64) ([]PelicanQuestionGeneratorKey, error) {
	if s.questionGenerator == nil || s.questionGenerator.keys == nil {
		return []PelicanQuestionGeneratorKey{}, nil
	}
	query = strings.TrimSpace(query)
	if len(query) > 200 {
		return nil, infraerrors.BadRequest("INVALID_PELICAN_GROUP_TEST", "key search is too long")
	}
	keys, err := s.questionGenerator.keys.SearchAPIKeys(ctx, 0, query, 100)
	if err != nil {
		return nil, err
	}
	result := make([]PelicanQuestionGeneratorKey, 0, len(keys)+1)
	selectedFound := false
	for i := range keys {
		key := &keys[i]
		selectedFound = selectedFound || key.ID == selectedID
		result = append(result, pelicanQuestionKeyOption(key))
	}
	if selectedID > 0 && !selectedFound {
		if key, err := s.questionGenerator.keys.GetByID(ctx, selectedID); err == nil && key != nil {
			result = append(result, pelicanQuestionKeyOption(key))
		} else if err != nil && !errors.Is(err, ErrAPIKeyNotFound) {
			return nil, err
		}
	}
	return result, nil
}

func pelicanQuestionKeyOption(key *APIKey) PelicanQuestionGeneratorKey {
	return PelicanQuestionGeneratorKey{ID: key.ID, Name: key.Name, UserID: key.UserID, Status: key.Status,
		Available: key.IsActive() && !key.IsExpired() && !key.IsQuotaExhausted()}
}

func normalizePelicanGroupFallbacks(legacy string, inputs []string) (string, []string, error) {
	bad := func() (string, []string, error) {
		return "", nil, infraerrors.BadRequest("INVALID_PELICAN_GROUP_TEST", "provide 1–20 nonempty fallback questions (32000 bytes each, 128000 bytes total)")
	}
	if inputs == nil {
		prompt := strings.TrimSpace(legacy)
		if prompt == "" || len(prompt) > pelicanQuestionMaxBytes || !utf8.ValidString(prompt) {
			return bad()
		}
		return prompt, nil, nil // Preserve old fixed-question plans.
	}
	if len(inputs) < 1 || len(inputs) > pelicanQuestionMaxFallbacks {
		return bad()
	}
	fallbacks := make([]string, len(inputs))
	total := 0
	for i, input := range inputs {
		fallbacks[i] = strings.TrimSpace(input)
		total += len(fallbacks[i])
		if fallbacks[i] == "" || len(fallbacks[i]) > pelicanQuestionMaxBytes || !utf8.ValidString(fallbacks[i]) || total > 128000 {
			return bad()
		}
	}
	return fallbacks[0], fallbacks, nil
}

func (s *PelicanGroupTestService) validateQuestionGenerator(ctx context.Context, input *PelicanQuestionGeneratorConfig, current *PelicanGroupTestPlan) (*PelicanQuestionGeneratorConfig, error) {
	if input == nil {
		return nil, nil
	}
	copy := *input
	copy.ModelID, copy.Prompt = strings.TrimSpace(copy.ModelID), strings.TrimSpace(copy.Prompt)
	if copy.Prompt == "" {
		copy.Prompt = DefaultPelicanQuestionGeneratorPrompt
	}
	if copy.APIKeyID <= 0 || copy.ModelID == "" || len(copy.ModelID) > 100 || len(copy.Prompt) > pelicanQuestionMaxBytes || !utf8.ValidString(copy.Prompt) {
		return nil, infraerrors.BadRequest("INVALID_PELICAN_GROUP_TEST", "generator key and model are required (100/32000 byte limits)")
	}
	// A revoked/deleted key in an existing plan may remain configured while the
	// operator adjusts its fallback questions. Runtime admission still fails.
	if current != nil && current.PelicanConfig != nil && current.PelicanConfig.QuestionGenerator != nil && current.PelicanConfig.QuestionGenerator.APIKeyID == copy.APIKeyID {
		return &copy, nil
	}
	if s.questionGenerator == nil || s.questionGenerator.keys == nil {
		return nil, infraerrors.BadRequest("INVALID_PELICAN_GROUP_TEST", "question generator is unavailable")
	}
	key, err := s.questionGenerator.keys.GetByID(ctx, copy.APIKeyID)
	if err != nil && !errors.Is(err, ErrAPIKeyNotFound) {
		return nil, err
	}
	if err != nil || key == nil {
		return nil, infraerrors.BadRequest("INVALID_PELICAN_GROUP_TEST", "generator API key does not exist")
	}
	return &copy, nil
}

func (s *PelicanGroupTestService) prepareGroupQuestion(ctx context.Context, original *PelicanTestConfig) (*PelicanTestConfig, error) {
	if original == nil {
		return nil, nil
	}
	resolved := *original
	resolved.FallbackPrompts = append([]string(nil), original.FallbackPrompts...)
	resolved.QuestionSource, resolved.QuestionGenerationError, resolved.QuestionGenerationMS = "", "", 0
	if original.QuestionGenerator != nil {
		input := *original.QuestionGenerator
		resolved.QuestionGenerator = &input
		started := time.Now()
		output, err := s.generateGroupQuestion(ctx, &input)
		resolved.QuestionGenerationMS = time.Since(started).Milliseconds()
		if ctx.Err() != nil {
			return &resolved, errors.New("question_generation_cancelled")
		}
		if err == nil {
			resolved.Prompt, resolved.QuestionSource = output, "ai"
			return &resolved, nil
		}
		resolved.QuestionSource, resolved.QuestionGenerationError = "fallback", pelicanQuestionFailureCode(err)
	} else if len(original.FallbackPrompts) > 0 {
		resolved.QuestionSource = "pool"
	} else {
		return &resolved, nil // Legacy fixed question and timing are unchanged.
	}
	fallbacks := resolved.FallbackPrompts
	if len(fallbacks) == 0 && strings.TrimSpace(original.Prompt) != "" {
		fallbacks = []string{original.Prompt}
	}
	if len(fallbacks) == 0 {
		return &resolved, errors.New("question_fallback_unavailable")
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(fallbacks))))
	if err != nil {
		return &resolved, errors.New("question_random_selection_failed")
	}
	resolved.Prompt = fallbacks[index.Int64()]
	return &resolved, nil
}

func (s *PelicanGroupTestService) generateGroupQuestion(ctx context.Context, input *PelicanQuestionGeneratorConfig) (string, error) {
	if s.questionGenerator == nil || s.questionGenerator.request == nil {
		return "", errors.New("generator_unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, pelicanQuestionTimeout)
	defer cancel()
	output, err := s.questionGenerator.request(ctx, input)
	if ctx.Err() != nil {
		return "", errors.New("generator_timeout")
	}
	if err != nil {
		return "", err
	}
	output = strings.TrimSpace(output)
	if output == "" || len(output) > pelicanQuestionMaxBytes || !utf8.ValidString(output) || strings.Contains(output, "```") || pelicanHTMLPattern.MatchString(output) {
		return "", errors.New("generator_invalid_output")
	}
	return output, nil
}

// Never persist raw HTTP bodies, transport errors, URLs or credentials.
func pelicanQuestionFailureCode(err error) string {
	switch err.Error() {
	case "generator_unavailable", "generator_timeout", "generator_key_unavailable", "generator_invalid_output", "generator_incomplete_response", "generator_response_too_large", "generator_request_failed":
		return err.Error()
	}
	if strings.HasPrefix(err.Error(), "generator_http_") {
		if code, e := strconv.Atoi(strings.TrimPrefix(err.Error(), "generator_http_")); e == nil && code >= 100 && code <= 599 {
			return fmt.Sprintf("generator_http_%d", code)
		}
	}
	return "generator_request_failed"
}

func requestPelicanQuestion(ctx context.Context, keys pelicanQuestionKeys, client *http.Client, endpoint string, input *PelicanQuestionGeneratorConfig) (string, error) {
	if keys == nil {
		return "", errors.New("generator_key_unavailable")
	}
	key, err := keys.GetByID(ctx, input.APIKeyID)
	if err != nil || key == nil || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || key.Key == "" {
		return "", errors.New("generator_key_unavailable")
	}
	body, err := json.Marshal(map[string]any{"model": input.ModelID, "messages": []map[string]string{{"role": "user", "content": input.Prompt}}, "stream": true, "max_tokens": 2048})
	if err != nil {
		return "", errors.New("generator_request_failed")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("generator_request_failed")
	}
	req.Header.Set("Authorization", "Bearer "+key.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", "sub2api-degradation-question-generator/1.0")
	// This is a normal authenticated, billable call through all gateway checks.
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("generator_request_failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("generator_http_%d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: pelicanQuestionMaxWireBytes + 1}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		data, err := io.ReadAll(limited)
		if err != nil {
			return "", errors.New("generator_request_failed")
		}
		if len(data) > pelicanQuestionMaxWireBytes {
			return "", errors.New("generator_response_too_large")
		}
		var response struct {
			Choices []struct {
				Message struct {
					Content string
					Refusal string
				}
				FinishReason string `json:"finish_reason"`
			}
		}
		if json.Unmarshal(data, &response) != nil || len(response.Choices) != 1 || response.Choices[0].FinishReason != "stop" || response.Choices[0].Message.Refusal != "" {
			return "", errors.New("generator_incomplete_response")
		}
		return response.Choices[0].Message.Content, nil
	}
	return readPelicanQuestionStream(limited)
}

func readPelicanQuestionStream(reader *io.LimitedReader) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 65536)
	var output strings.Builder
	finished, done := false, false
	data := []string{}
	dispatch := func() error {
		raw := strings.Join(data, "\n")
		data = nil
		if raw == "" {
			return nil
		}
		if raw == "[DONE]" {
			done = true
			return nil
		}
		var event struct {
			Error   json.RawMessage
			Choices []struct {
				Index int
				Delta struct {
					Content string
					Refusal string
				}
				FinishReason *string `json:"finish_reason"`
			}
		}
		if json.Unmarshal([]byte(raw), &event) != nil || (len(event.Error) > 0 && string(event.Error) != "null") {
			return errors.New("generator_invalid_output")
		}
		for _, choice := range event.Choices {
			if choice.Index != 0 || choice.Delta.Refusal != "" {
				return errors.New("generator_invalid_output")
			}
			if finished && choice.Delta.Content != "" {
				return errors.New("generator_invalid_output")
			}
			output.WriteString(choice.Delta.Content)
			if output.Len() > pelicanQuestionMaxBytes {
				return errors.New("generator_response_too_large")
			}
			if choice.FinishReason != nil {
				if *choice.FinishReason != "stop" {
					return errors.New("generator_incomplete_response")
				}
				finished = true
			}
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		if line == "" {
			if err := dispatch(); err != nil {
				return "", err
			}
			if done {
				break
			}
		}
	}
	if !done && len(data) > 0 {
		if err := dispatch(); err != nil {
			return "", err
		}
	}
	if scanner.Err() != nil || reader.N <= 0 {
		return "", errors.New("generator_response_too_large")
	}
	if !done || !finished {
		return "", errors.New("generator_incomplete_response")
	}
	return output.String(), nil
}
