package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const GroqModel = "whisper-large-v3-turbo"

var (
	groqBaseURL = "https://api.groq.com/openai/v1"
	groqClient  = &http.Client{Timeout: 60 * time.Second}
)

// CloudError is a cloud transcription failure whose message is written for the user.
type CloudError struct {
	Message string
	Cause   error
}

func (e *CloudError) Error() string { return e.Message }
func (e *CloudError) Unwrap() error { return e.Cause }

var ErrNoGroqKey = &CloudError{Message: "Add your free Groq API key in Settings › Advanced to start dictating"}

// TranscribeGroq sends one recording to Groq's hosted Whisper.
func TranscribeGroq(apiKey string, samples []int16, language, prompt string) (string, error) {
	return TranscribeGroqContext(context.Background(), apiKey, samples, language, prompt)
}

func TranscribeGroqContext(ctx context.Context, apiKey string, samples []int16, language, prompt string) (string, error) {
	if apiKey == "" {
		return "", ErrNoGroqKey
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	file.Write(wavBytes(samples, 16000))
	form.WriteField("model", GroqModel)
	form.WriteField("response_format", "json")
	form.WriteField("temperature", "0")
	if language != "" && language != "auto" {
		form.WriteField("language", language)
	}
	if prompt != "" {
		form.WriteField("prompt", prompt)
	}
	form.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqBaseURL+"/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	data, err := groqDo(req, apiKey)
	if err != nil {
		return "", err
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("unexpected response from Groq: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

// CheckGroqKey lists models, which proves the key without spending audio quota.
func CheckGroqKey(apiKey string) error {
	req, err := http.NewRequest(http.MethodGet, groqBaseURL+"/models", nil)
	if err != nil {
		return err
	}
	_, err = groqDo(req, apiKey)
	return err
}

func groqDo(req *http.Request, apiKey string) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := groqClient.Do(req)
	if err != nil {
		return nil, &CloudError{Message: "Couldn't reach Groq to transcribe — check your internet connection", Cause: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, &CloudError{Message: "The connection to Groq dropped — try again", Cause: err}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, groqError(resp.StatusCode, resp.Header.Get("Retry-After"), data)
	}
	return data, nil
}

func groqError(status int, retryAfter string, body []byte) error {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &parsed)
	detail := parsed.Error.Message
	if detail == "" {
		detail = strings.TrimSpace(string(body))
	}
	cause := fmt.Errorf("groq status %d: %s", status, detail)

	var msg string
	switch {
	case status == http.StatusTooManyRequests:
		msg = "Groq's free transcription limit is used up"
		if wait := retryWait(retryAfter, detail); wait > 0 {
			msg += " — it resets in " + formatWait(wait)
		} else {
			msg += " for now"
		}
		msg += ". Try again then, or switch to on-device transcription in Settings › Advanced."
	case status == http.StatusUnauthorized:
		msg = "Groq rejected your API key — check it in Settings › Advanced"
	case status == http.StatusRequestEntityTooLarge:
		msg = "This recording is too long for Groq — keep dictations under about 10 minutes"
	case status >= 500:
		msg = fmt.Sprintf("Groq's transcription service is having trouble (error %d) — try again in a moment", status)
	default:
		if len(detail) > 120 {
			detail = detail[:120] + "…"
		}
		msg = fmt.Sprintf("Groq couldn't transcribe this (error %d): %s", status, detail)
	}
	return &CloudError{Message: msg, Cause: cause}
}

var tryAgainIn = regexp.MustCompile(`try again in ((?:[0-9.]+(?:ms|h|m|s))+)`)

// retryWait reads the Retry-After header, else the "try again in 7m12.5s" in Groq's message.
func retryWait(header, message string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if m := tryAgainIn.FindStringSubmatch(message); m != nil {
		if d, err := time.ParseDuration(m[1]); err == nil {
			return d
		}
	}
	return 0
}

func formatWait(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d seconds", max(1, int(d.Round(time.Second).Seconds())))
	case d < time.Hour:
		m := int((d + time.Minute - 1) / time.Minute)
		if m == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", m)
	default:
		h, m := int(d/time.Hour), int(d%time.Hour/time.Minute)
		if m == 0 {
			return fmt.Sprintf("%d h", h)
		}
		return fmt.Sprintf("%d h %d min", h, m)
	}
}
