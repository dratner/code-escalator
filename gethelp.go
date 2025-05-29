package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/sashabaranov/go-openai"
)

type GetHelpTool struct {
	summaryPath string
	modelName   string
}

func NewGetHelpTool(summaryPath, modelName string) *GetHelpTool {
	return &GetHelpTool{
		summaryPath: summaryPath,
		modelName:   modelName,
	}
}

func (t *GetHelpTool) Name() string {
	return "get_help"
}

func (t *GetHelpTool) Description() string {
	return "Escalate difficult problems to OpenAI for expert guidance"
}

func (t *GetHelpTool) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question": map[string]interface{}{
				"type":        "string",
				"description": "The specific question or problem you need help with",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Brief summary of your project context",
			},
			"relevant_code": map[string]interface{}{
				"type":        "string",
				"description": "Any relevant code snippets (optional)",
			},
		},
		"required": []string{"question", "summary"},
	}
}

func (t *GetHelpTool) Call(arguments map[string]interface{}) ([]map[string]interface{}, error) {
	var question, summary, relevantCode string
	
	if q, ok := arguments["question"].(string); ok {
		question = q
	}
	if s, ok := arguments["summary"].(string); ok {
		summary = s
	}
	if rc, ok := arguments["relevant_code"].(string); ok {
		relevantCode = rc
	}

	if question == "" || summary == "" {
		return []map[string]interface{}{
			{
				"type": "text",
				"text": "Error: Missing required fields: question and summary",
			},
		}, fmt.Errorf("missing required fields")
	}

	// Load project summary
	projectSummary, err := t.loadSummary()
	if err != nil {
		log.Printf("Couldn't load the summary file: %v", err)
		return []map[string]interface{}{
			{
				"type": "text",
				"text": "The architect is currently unavailable. Please try again later.",
			},
		}, err
	}

	// Build prompt
	prompt, err := t.buildPrompt(projectSummary, question, relevantCode)
	if err != nil {
		log.Printf("Couldn't build the prompt: %v", err)
		return []map[string]interface{}{
			{
				"type": "text",
				"text": "The architect is currently unavailable. Please try again later.",
			},
		}, err
	}

	log.Println("Ready to call OpenAI")

	// Call OpenAI
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	answer, err := t.askOpenAI(ctx, prompt)
	if err != nil {
		log.Printf("OpenAI call failed: %v", err)
		return []map[string]interface{}{
			{
				"type": "text",
				"text": "The architect is currently unavailable. Please try again later.",
			},
		}, err
	}

	log.Printf("[%s] OpenAI call completed successfully", time.Now().Format(time.RFC3339))
	
	return []map[string]interface{}{
		{
			"type": "text",
			"text": answer,
		},
	}, nil
}

func (t *GetHelpTool) loadSummary() (string, error) {
	path := t.summaryPath
	if path == "" {
		path = "./README.md"
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (t *GetHelpTool) buildPrompt(summary, question, relevantCode string) (string, error) {
	template := `As a software architect, provide help with this issue:

<summary>
%s
</summary>

---
**Question:** %s

**Relevant Code:** %s`

	prompt := fmt.Sprintf(template, summary, question, relevantCode)

	// Check token limit (rough estimate: ~4 chars per token)
	if len(prompt) > 80000 { // 20,000 tokens * 4 chars
		return "", fmt.Errorf("prompt exceeds 20,000 token limit")
	}

	return prompt, nil
}

func (t *GetHelpTool) askOpenAI(ctx context.Context, prompt string) (string, error) {
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	maxRetries := 3
	backoffDurations := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

	model := t.modelName
	if model == "" {
		model = "o3" // default
	}

	for attempt := range maxRetries {
		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		})

		if err != nil {
			// Check if it's a retryable error (429 or 5xx)
			if attempt < maxRetries-1 {
				time.Sleep(backoffDurations[attempt])
				continue
			}
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no response from OpenAI")
		}

		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("max retries exceeded")
}