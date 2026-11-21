package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// apiPath is the endpoint of the decisions API.
	apiPath = "/v1/decisions"

	// questionTypeChoice selects one option from a defined set.
	questionTypeChoice = "choice"
	// questionTypeNoul answers a yes/no question with a probability.
	questionTypeNoul = "noul"

	// maxErrorBody bounds how much of a failed response body is kept in an error.
	maxErrorBody = 512
)

// question is one typed question sent to a decisions endpoint.
type question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

// decisionsRequest is the body of a decisions call.
type decisionsRequest struct {
	Model     string              `json:"model"`
	State     decisionsState      `json:"state"`
	Questions map[string]question `json:"questions"`
}

// decisionsState is the state all questions are evaluated against.
type decisionsState struct {
	Context string       `json:"context,omitempty"`
	Message stateMessage `json:"message"`
}

// stateMessage carries the message text.
type stateMessage struct {
	Text string `json:"text"`
}

// Response is the answer payload of a decisions call.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// Answer holds the typed answer of one question.
//
// Fields unused by the answer's type decode to their zero values. Noul is
// nil when the response omits or nulls it, so a missing probability is
// distinguishable from a valid zero.
type Answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// Usage reports token consumption of a decisions call.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Marshal implements [cache.Item] so answers can be cached.
func (r *Response) Marshal() ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return data, nil
}

// Unmarshal implements [cache.Item] so answers can be cached.
func (r *Response) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, r); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// ask sends one decisions request and decodes the answers.
func (p *Plugin) ask(ctx context.Context, req decisionsRequest) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal request: %w", ErrRequestFailed, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrRequestFailed, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"%w: HTTP %d: %s",
			ErrRequestFailed,
			resp.StatusCode,
			readErrorSnippet(resp.Body),
		)
	}

	out := new(Response)
	if err = json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("%w: decode response: %w", ErrRequestFailed, err)
	}

	return out, nil
}

// readErrorSnippet reads a bounded prefix of a failed response body.
func readErrorSnippet(r io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(r, maxErrorBody))
	if err != nil {
		return "<unreadable body>"
	}

	return strings.TrimSpace(string(data))
}
