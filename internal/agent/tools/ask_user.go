package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"StarCore/internal/agent"
)

// AskUserRequest represents a pending question to the user.
type AskUserRequest struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

// AskUserResponse is the user's reply.
type AskUserResponse struct {
	ID     string `json:"id"`
	Answer string `json:"answer"`
}

// AskUserRegistry manages pending ask_user requests.
type AskUserRegistry struct {
	mu      sync.Mutex
	pending map[string]chan AskUserResponse
	counter int
}

var AskUserReg = &AskUserRegistry{pending: make(map[string]chan AskUserResponse)}

// Submit sends a question to the frontend and blocks until the user responds.
// Returns the user's answer or an error on timeout/cancellation.
func (r *AskUserRegistry) Submit(ctx context.Context, question string, options []string) (string, error) {
	r.mu.Lock()
	r.counter++
	id := fmt.Sprintf("ask_%d", r.counter)
	ch := make(chan AskUserResponse, 1)
	r.pending[id] = ch
	r.mu.Unlock()

	// Emit event to frontend (handled by the caller via AskUserReg.NotifyCh or polling)
	req := AskUserRequest{ID: id, Question: question, Options: options}

	// Signal anyone listening that there's a new request
	select {
	case askUserNotifyCh <- req:
	default:
	}

	// Wait for response with timeout
	select {
	case resp := <-ch:
		return resp.Answer, nil
	case <-ctx.Done():
		r.mu.Lock()
		delete(r.pending, id)
		r.mu.Unlock()
		return "", ctx.Err()
	case <-time.After(5 * time.Minute):
		r.mu.Lock()
		delete(r.pending, id)
		r.mu.Unlock()
		return "用户未在5分钟内回复，请基于你的最佳判断继续。", nil
	}
}

// Respond is called by the backend (from Wails) when the user answers.
func (r *AskUserRegistry) Respond(response AskUserResponse) bool {
	r.mu.Lock()
	ch, ok := r.pending[response.ID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	ch <- response
	return true
}

// askUserNotifyCh is a buffered channel used to notify the Service that
// a new ask_user request is pending (so it can emit to the frontend).
var askUserNotifyCh = make(chan AskUserRequest, 8)

// PollAskUserRequests returns a channel that receives new ask_user requests.
// The caller should read from this and emit events to the frontend.
func PollAskUserRequests() <-chan AskUserRequest {
	return askUserNotifyCh
}

// AskUserTool allows the AI to ask the user clarifying questions mid-task.
type AskUserTool struct{}

func NewAskUserTool() *AskUserTool { return &AskUserTool{} }

func (t *AskUserTool) ID() string             { return "ask_user" }
func (t *AskUserTool) Name() string           { return "Ask User" }
func (t *AskUserTool) RequiresApproval() bool { return false }

func (t *AskUserTool) Description() string {
	return "向用户提问以澄清需求。仅在需求模糊、有多种方案、或需要用户决策时使用。不要用于简单的确认。"
}

func (t *AskUserTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"question": {Type: "string", Description: "The clarifying question to ask the user. Be specific and concise."},
			"options":  {Type: "array", Description: "Optional: 2-4 predefined options for the user to choose from. Each option string should describe the choice and its trade-offs."},
		},
		Required: []string{"question"},
	}
}

func (t *AskUserTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	question, ok := args["question"].(string)
	question = strings.TrimSpace(question)
	if !ok || question == "" {
		return "", fmt.Errorf("question is required")
	}

	var options []string
	if raw, ok := args["options"]; ok {
		switch v := raw.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					options = append(options, s)
				}
			}
		case []string:
			options = v
		}
	}

	answer, err := AskUserReg.Submit(ctx, question, options)
	if err != nil {
		return "", err
	}
	return "用户回复: " + answer, nil
}
