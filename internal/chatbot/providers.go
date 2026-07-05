package chatbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// llmMsg is a provider-agnostic chat turn. Role is "user" or "assistant" —
// each provider function below translates it into whatever shape that
// provider's API expects.
type llmMsg struct {
	Role    string
	Content string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// callLLM dispatches to whichever provider is configured via AI_PROVIDER
// (gemini | grok | huggingface). Defaults to Gemini if unset/unrecognized,
// so switching providers is a single env var change — no code/redeploy of
// call sites needed.
func callLLM(systemPrompt string, messages []llmMsg) (string, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER"))) {
	case "grok":
		return callGrok(systemPrompt, messages)
	case "huggingface", "hf":
		return callHuggingFace(systemPrompt, messages)
	default: // "gemini" or unset
		return callGemini(systemPrompt, messages)
	}
}

// ── Gemini (Google Generative Language API) ──────────────────────────────────

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiReq struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  struct {
		MaxOutputTokens int `json:"maxOutputTokens"`
	} `json:"generationConfig"`
}

type geminiResp struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func callGemini(systemPrompt string, messages []llmMsg) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash"
	}

	contents := make([]geminiContent, 0, len(messages))
	for _, m := range messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model" // Gemini calls the assistant turn "model", not "assistant"
		}
		contents = append(contents, geminiContent{Role: role, Parts: []geminiPart{{Text: m.Content}}})
	}

	reqBody := geminiReq{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: systemPrompt}}},
		Contents:          contents,
	}
	reqBody.GenerationConfig.MaxOutputTokens = 1024

	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var gr geminiResp
	json.NewDecoder(resp.Body).Decode(&gr)
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini (status %d)", resp.StatusCode)
	}
	return gr.Candidates[0].Content.Parts[0].Text, nil
}

// ── Grok (xAI) and Hugging Face — both speak the OpenAI chat-completions format ──

type openAIChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatReq struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openAIChatMsg `json:"messages"`
}

type openAIChatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callOpenAICompatible(baseURL, apiKey, model, providerName, systemPrompt string, messages []llmMsg) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("%s API key not set", providerName)
	}

	chatMsgs := make([]openAIChatMsg, 0, len(messages)+1)
	chatMsgs = append(chatMsgs, openAIChatMsg{Role: "system", Content: systemPrompt})
	for _, m := range messages {
		chatMsgs = append(chatMsgs, openAIChatMsg{Role: m.Role, Content: m.Content})
	}

	body, _ := json.Marshal(openAIChatReq{Model: model, MaxTokens: 1024, Messages: chatMsgs})

	req, err := http.NewRequest("POST", strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var cr openAIChatResp
	json.NewDecoder(resp.Body).Decode(&cr)
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("empty response from %s (status %d)", providerName, resp.StatusCode)
	}
	return cr.Choices[0].Message.Content, nil
}

func callGrok(systemPrompt string, messages []llmMsg) (string, error) {
	model := os.Getenv("GROK_MODEL")
	if model == "" {
		model = "grok-4" // verify against current xAI model list at https://docs.x.ai — slugs change over time
	}
	return callOpenAICompatible("https://api.x.ai/v1", os.Getenv("GROK_API_KEY"), model, "Grok", systemPrompt, messages)
}

func callHuggingFace(systemPrompt string, messages []llmMsg) (string, error) {
	model := os.Getenv("HUGGINGFACE_MODEL")
	if model == "" {
		model = "meta-llama/Llama-3.1-8B-Instruct" // verify current availability at https://huggingface.co/models
	}
	// HF's router endpoint is OpenAI-chat-completions compatible and auto-routes
	// the request to whichever inference provider is hosting the given model.
	return callOpenAICompatible("https://router.huggingface.co/v1", os.Getenv("HUGGINGFACE_API_KEY"), model, "Hugging Face", systemPrompt, messages)
}
