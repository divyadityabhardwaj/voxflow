package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	retryMaxAttempts = 3
	retryBaseDelay   = 250 * time.Millisecond
	retryMaxDelay    = 5 * time.Second
	// A longer wait means a quota window, not a blip: give up and paste raw text.
	maxRetryAfter = 5 * time.Second
	maxErrorLen   = 200
)

var (
	ErrRateLimited = errors.New("rate limit reached")
	ErrNoAPIKey    = errors.New("API key not set")
)

// A cold local server loads the model before answering, which can take tens of seconds.
const (
	localRequestTimeout = 30 * time.Second
	localPrewarmTimeout = 60 * time.Second
)

// Bounds a dictation-path refine: enough for a slow model on a long dictation,
// short enough that a hung provider falls back to raw text within seconds.
func RefineBudget(rawText string) time.Duration {
	perWord := time.Duration(len(strings.Fields(rawText))) * 40 * time.Millisecond
	return min(10*time.Second+perWord, 20*time.Second)
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusBadGateway ||
		code == http.StatusServiceUnavailable
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	// OpenRouter-only: turn off hybrid-thinking so the reply is just the text.
	Reasoning       *reasoningOpts  `json:"reasoning,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	ResponseFormat  *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type reasoningOpts struct {
	Enabled bool `json:"enabled"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type OpenAIClient struct {
	Provider   Provider
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	// RefineTimeout replaces RefineBudget, for servers that may have to load the model first.
	RefineTimeout time.Duration
}

func (c *OpenAIClient) reasoning() *reasoningOpts {
	if c.Provider.DisableReasoning {
		return &reasoningOpts{Enabled: false}
	}
	return nil
}

// Lowest thinking Gemini Flash accepts: it halved latency with no quality loss in
// scripts/bench-refine. gpt-oss at "low" was only ~100ms faster and left fillers
// in and cleaned less, so it keeps its default.
func (c *OpenAIClient) reasoningEffort(model string) string {
	switch {
	case c.Provider.ID != "gemini" || !strings.Contains(model, "flash"):
		return ""
	case strings.HasPrefix(model, "gemini-2.5"):
		return "none"
	case strings.HasPrefix(model, "gemini-3"):
		return "minimal" // 3.x can't turn thinking off
	}
	return ""
}

func (c *OpenAIClient) jsonMode() *responseFormat {
	if c.Provider.JSONMode {
		return &responseFormat{Type: "json_object"}
	}
	return nil
}

func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 4,
		MaxIdleConns:        8,
		IdleConnTimeout:     120 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
}

func NewClient(p Provider, apiKey string) *OpenAIClient {
	c := &OpenAIClient{
		Provider: p,
		BaseURL:  p.BaseURL,
		APIKey:   apiKey,
		HTTPClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: NewTransport(),
		},
	}
	if p.Local {
		c.HTTPClient.Timeout = localRequestTimeout
		c.RefineTimeout = localRequestTimeout
	}
	return c
}

// serverURL is the local server's root; /v1 is appended.
func (c *OpenAIClient) SetServerURL(serverURL string) {
	c.BaseURL = strings.TrimRight(serverURL, "/") + "/v1"
}

func (c *OpenAIClient) missingKey() bool {
	return c.Provider.NeedsKey && c.APIKey == ""
}

func DoWithRetry(hc *http.Client, newReq func() (*http.Request, error)) ([]byte, int, error) {
	return DoWithRetryContext(context.Background(), hc, retryMaxAttempts, newReq)
}

// Retries dropped connections, 502/503 and short 429s up to retries times, never
// sleeping past ctx's deadline. Non-2xx bodies come back with a nil error; turn
// them into an error with StatusError.
func DoWithRetryContext(ctx context.Context, hc *http.Client, retries int, newReq func() (*http.Request, error)) ([]byte, int, error) {
	delay := retryBaseDelay
	for attempt := 0; ; attempt++ {
		req, err := newReq()
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := hc.Do(req.WithContext(ctx))
		if err != nil {
			if attempt == retries || isFinalTransportError(err) || !sleepWithin(ctx, delay) {
				return nil, 0, fmt.Errorf("failed to send request: %w", err)
			}
			delay = min(delay*2, retryMaxDelay)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
		}

		wait := delay
		if resp.StatusCode == http.StatusTooManyRequests {
			ra := retryAfter(resp.Header)
			if ra > maxRetryAfter || quotaExhausted(body) {
				return body, resp.StatusCode, nil
			}
			wait = max(wait, ra)
		}
		if !isRetryableStatus(resp.StatusCode) || attempt == retries || !sleepWithin(ctx, wait) {
			return body, resp.StatusCode, nil
		}
		delay = min(delay*2, retryMaxDelay)
	}
}

// Offline (DNS/dial failure) or already timed out: another attempt fails the same way, only later.
func isFinalTransportError(err error) bool {
	var ne net.Error
	var op *net.OpError
	return (errors.As(err, &ne) && ne.Timeout()) || (errors.As(err, &op) && op.Op == "dial")
}

// Sleeps d unless that would leave the retry under a second before ctx's deadline.
func sleepWithin(ctx context.Context, d time.Duration) bool {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < d+time.Second {
		return false
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func retryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		return time.Duration(secs * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t)
	}
	return 0
}

func quotaExhausted(body []byte) bool {
	b := strings.ToLower(string(body))
	for _, s := range []string{"quota", "per day", "per-day", "daily"} {
		if strings.Contains(b, s) {
			return true
		}
	}
	return false
}

// Short error for a non-2xx reply. Bodies can echo the user's text, so only the
// provider's own message (or the start of the body) is kept.
func StatusError(status int, body []byte) error {
	if status == http.StatusTooManyRequests {
		return ErrRateLimited
	}
	msg := providerMessage(body)
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if len(msg) > maxErrorLen {
		msg = strings.ToValidUTF8(msg[:maxErrorLen], "") + "…"
	}
	if msg == "" {
		return fmt.Errorf("API error (status %d)", status)
	}
	return fmt.Errorf("API error (status %d): %s", status, msg)
}

// {"error":{"message":"..."}} (OpenAI-style), [{"error":{...}}] (Gemini's OpenAI
// endpoint) or {"error":"..."} (Ollama).
func providerMessage(body []byte) string {
	var env struct {
		Error json.RawMessage `json:"error"`
	}
	var list []json.RawMessage
	if json.Unmarshal(body, &list) == nil && len(list) > 0 {
		body = list[0]
	}
	if json.Unmarshal(body, &env) != nil || len(env.Error) == 0 {
		return ""
	}
	var obj struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(env.Error, &obj) == nil && obj.Message != "" {
		return obj.Message
	}
	var str string
	_ = json.Unmarshal(env.Error, &str)
	return str
}

func (c *OpenAIClient) doPost(ctx context.Context, retries int, reqBody []byte, url string) ([]byte, int, error) {
	return DoWithRetryContext(ctx, c.HTTPClient, retries, func() (*http.Request, error) {
		req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
		if err != nil {
			return nil, err
		}
		c.applyHeaders(req)
		return req, nil
	})
}

func (c *OpenAIClient) doGet(url string) ([]byte, int, error) {
	return DoWithRetry(c.HTTPClient, func() (*http.Request, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		c.applyHeaders(req)
		return req, nil
	})
}

func (c *OpenAIClient) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range c.Provider.Headers {
		req.Header.Set(k, v)
	}
}

func refineMessages(rawText string) []chatMessage {
	return []chatMessage{
		{Role: "system", Content: BuildSystemPrompt()},
		{Role: "user", Content: "Transcription to refine:\n<transcription>\n" + rawText + "\n</transcription>"},
	}
}

func (c *OpenAIClient) RefineText(rawText, model string) (string, int, bool, error) {
	if c.missingKey() {
		return "", 0, false, ErrNoAPIKey
	}
	req := chatRequest{
		Model:           model,
		Messages:        refineMessages(rawText),
		Temperature:     0.3,
		MaxTokens:       RefineMaxTokens(rawText),
		Reasoning:       c.reasoning(),
		ReasoningEffort: c.reasoningEffort(model),
		ResponseFormat:  c.jsonMode(),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", 0, false, fmt.Errorf("failed to marshal request: %w", err)
	}

	budget := RefineBudget(rawText)
	if c.RefineTimeout > 0 {
		budget = c.RefineTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	respBody, statusCode, err := c.doPost(ctx, 1, reqBody, url)
	if err != nil {
		return "", 0, false, err
	}

	if statusCode != http.StatusOK {
		return "", 0, false, StatusError(statusCode, respBody)
	}

	var apiResp chatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", 0, false, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", 0, false, fmt.Errorf("no response generated")
	}

	result := apiResp.Choices[0].Message.Content
	tokenCount := apiResp.Usage.CompletionTokens

	refined, okToGo, parsed := ParseRefineResponse(result, rawText)
	if !parsed {
		return rawText, tokenCount, false, nil
	}
	return refined, tokenCount, okToGo, nil
}

func (c *OpenAIClient) RetryWithInstruction(text, instruction, model string) (string, error) {
	if c.missingKey() {
		return "", ErrNoAPIKey
	}
	prompt := fmt.Sprintf(`Apply the following instruction to the text:
Instruction: %s

Text:
%s

Return ONLY the modified text, nothing else.`, instruction, text)

	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature:     0.3,
		Reasoning:       c.reasoning(),
		ReasoningEffort: c.reasoningEffort(model),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	respBody, statusCode, err := c.doPost(context.Background(), retryMaxAttempts, reqBody, url)
	if err != nil {
		return "", err
	}

	if statusCode != http.StatusOK {
		return "", StatusError(statusCode, respBody)
	}

	var apiResp chatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	return StripCodeFences(stripThink(apiResp.Choices[0].Message.Content)), nil
}

func (c *OpenAIClient) CheckModel(model string) (int64, float64, error) {
	if c.missingKey() {
		return 0, 0, ErrNoAPIKey
	}
	req := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: LatencyTestText},
		},
		Temperature:     0.3,
		Reasoning:       c.reasoning(),
		ReasoningEffort: c.reasoningEffort(model),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	startTime := time.Now()

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, 0, StatusError(resp.StatusCode, respBody)
	}

	var apiResp chatResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return latency, 0, nil // return latency even if TPS calculation fails
	}

	tokenCount := apiResp.Usage.CompletionTokens
	var tps float64
	if latency > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(latency) / 1000.0)
	}

	return latency, tps, nil
}

// Falls back to the provider's built-in list when the catalog can't be read.
func (c *OpenAIClient) GetModels() ([]string, error) {
	p := c.Provider
	if c.missingKey() && !p.OpenModels {
		return p.FallbackModels, ErrNoAPIKey
	}

	respBody, statusCode, err := c.doGet(c.BaseURL + "/models" + p.ModelsQuery)
	if err != nil {
		return p.FallbackModels, err
	}
	if statusCode != http.StatusOK {
		return p.FallbackModels, StatusError(statusCode, respBody)
	}

	var modResp modelsResponse
	if err := json.Unmarshal(respBody, &modResp); err != nil {
		return p.FallbackModels, fmt.Errorf("failed to parse models: %w", err)
	}

	var models []string
	for _, m := range modResp.Data {
		id := strings.TrimPrefix(m.ID, p.ModelPrefix)
		if p.KeepModel == nil || p.KeepModel(id) {
			models = append(models, id)
		}
	}
	if len(models) == 0 {
		return p.FallbackModels, nil
	}

	sort.Strings(models)
	return models, nil
}

// Opens the connection (or, for a local server, loads the model) while the user
// is still speaking. Fire-and-forget.
func (c *OpenAIClient) Prewarm(model string) {
	if c.missingKey() {
		return
	}
	if c.Provider.Local {
		c.prewarmLocal(model)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "HEAD", c.BaseURL, nil)
		if err != nil {
			return
		}
		c.applyHeaders(req)

		resp, err := c.HTTPClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
}

func (c *OpenAIClient) prewarmLocal(model string) {
	if model == "" {
		return
	}
	root := strings.TrimSuffix(c.BaseURL, "/v1")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), localPrewarmTimeout)
		defer cancel()
		if strings.Contains(root, ":11434") || strings.Contains(root, "ollama") {
			c.keepOllamaLoaded(ctx, root, model)
		}
		_ = c.WarmUp(ctx, model)
	}()
}

// Ollama unloads a model after 5 idle minutes and ignores keep_alive on its /v1 API.
func (c *OpenAIClient) keepOllamaLoaded(ctx context.Context, root, model string) {
	body, err := json.Marshal(map[string]string{"model": model, "keep_alive": "30m"})
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, "POST", root+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Transport: c.HTTPClient.Transport}).Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

// Asks for one token behind the refine system prompt, so a local server loads the
// model and caches the prompt prefix while the user is still speaking.
func (c *OpenAIClient) WarmUp(ctx context.Context, model string) error {
	reqBody, err := json.Marshal(chatRequest{
		Model:     model,
		Messages:  refineMessages(""),
		MaxTokens: 1,
		Reasoning: c.reasoning(),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	c.applyHeaders(req)
	// Not c.HTTPClient: a cold model load can outlast its timeout.
	resp, err := (&http.Client{Transport: c.HTTPClient.Transport}).Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}
