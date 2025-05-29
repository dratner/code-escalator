package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMCPServer_HandleInitialize(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0")
	result := server.HandleInitialize()

	if result["protocolVersion"] != "2025-03-26" {
		t.Errorf("Expected protocolVersion '2025-03-26', got %v", result["protocolVersion"])
	}

	serverInfo := result["serverInfo"].(map[string]string)
	if serverInfo["name"] != "test-server" {
		t.Errorf("Expected server name 'test-server', got %v", serverInfo["name"])
	}
	if serverInfo["version"] != "1.0.0" {
		t.Errorf("Expected server version '1.0.0', got %v", serverInfo["version"])
	}
}

func TestMCPServer_RegisterTool(t *testing.T) {
	server := NewMCPServer("test", "1.0.0")
	tool := NewGetHelpTool("", "gpt-4o")
	
	server.RegisterTool(tool)
	
	if len(server.tools) != 1 {
		t.Errorf("Expected 1 tool registered, got %d", len(server.tools))
	}
	
	if server.tools["get_help"] == nil {
		t.Error("Expected get_help tool to be registered")
	}
}

func TestMCPServer_HandleToolsList(t *testing.T) {
	server := NewMCPServer("test", "1.0.0")
	tool := NewGetHelpTool("", "gpt-4o")
	server.RegisterTool(tool)
	
	result := server.HandleToolsList()
	tools := result["tools"].([]map[string]interface{})
	
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool in list, got %d", len(tools))
	}
	
	if tools[0]["name"] != "get_help" {
		t.Errorf("Expected tool name 'get_help', got %v", tools[0]["name"])
	}
	
	if tools[0]["description"] == "" {
		t.Error("Expected non-empty description")
	}
	
	schema := tools[0]["inputSchema"].(map[string]interface{})
	if schema["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", schema["type"])
	}
}

func TestGetHelpTool_Name(t *testing.T) {
	tool := NewGetHelpTool("", "gpt-4o")
	if tool.Name() != "get_help" {
		t.Errorf("Expected name 'get_help', got %s", tool.Name())
	}
}

func TestGetHelpTool_Schema(t *testing.T) {
	tool := NewGetHelpTool("", "gpt-4o")
	schema := tool.Schema()
	
	if schema["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", schema["type"])
	}
	
	props := schema["properties"].(map[string]interface{})
	if props["question"] == nil {
		t.Error("Expected 'question' property in schema")
	}
	if props["summary"] == nil {
		t.Error("Expected 'summary' property in schema")
	}
	
	required := schema["required"].([]string)
	if len(required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(required))
	}
}

func TestGetHelpTool_Call_MissingFields(t *testing.T) {
	tool := NewGetHelpTool("", "gpt-4o")
	
	// Test missing question
	content, err := tool.Call(map[string]interface{}{
		"summary": "test summary",
	})
	
	if err == nil {
		t.Error("Expected error for missing question")
	}
	
	if len(content) == 0 || content[0]["type"] != "text" {
		t.Error("Expected error message in content")
	}
	
	// Test missing summary
	content, err = tool.Call(map[string]interface{}{
		"question": "test question",
	})
	
	if err == nil {
		t.Error("Expected error for missing summary")
	}
}

func TestGetHelpTool_LoadSummary_Default(t *testing.T) {
	tool := NewGetHelpTool("", "gpt-4o")
	content, err := tool.loadSummary()
	
	if err != nil {
		t.Errorf("Expected no error loading default summary, got: %v", err)
	}
	
	if !strings.Contains(content, "MCP Escalator") {
		t.Error("Expected content to contain 'MCP Escalator'")
	}
}

func TestGetHelpTool_LoadSummary_Custom(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test-summary-*.md")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	
	testContent := "# Custom Summary\nThis is a test summary."
	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	
	tool := NewGetHelpTool(tmpFile.Name(), "gpt-4o")
	content, err := tool.loadSummary()
	
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	if content != testContent {
		t.Errorf("Expected %q, got %q", testContent, content)
	}
}

func TestGetHelpTool_LoadSummary_Missing(t *testing.T) {
	tool := NewGetHelpTool("nonexistent.md", "gpt-4o")
	_, err := tool.loadSummary()
	
	if err == nil {
		t.Error("Expected error for missing file")
	}
}

func TestGetHelpTool_BuildPrompt(t *testing.T) {
	tool := NewGetHelpTool("", "gpt-4o")
	
	summary := "# Test Project\nThis is a test."
	question := "How do I test this?"
	relevantCode := "func test() {}"
	
	prompt, err := tool.buildPrompt(summary, question, relevantCode)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	
	if !strings.Contains(prompt, "software architect") {
		t.Error("Expected prompt to contain 'software architect'")
	}
	if !strings.Contains(prompt, summary) {
		t.Error("Expected prompt to contain summary")
	}
	if !strings.Contains(prompt, question) {
		t.Error("Expected prompt to contain question")
	}
	if !strings.Contains(prompt, relevantCode) {
		t.Error("Expected prompt to contain relevant code")
	}
}

func TestGetHelpTool_BuildPrompt_TokenLimit(t *testing.T) {
	tool := NewGetHelpTool("", "gpt-4o")
	
	// Create very long summary
	longSummary := strings.Repeat("a", 85000)
	
	_, err := tool.buildPrompt(longSummary, "test", "test")
	if err == nil {
		t.Error("Expected error for prompt exceeding token limit")
	}
	
	if !strings.Contains(err.Error(), "20,000 token limit") {
		t.Errorf("Expected token limit error, got: %v", err)
	}
}

func TestGetHelpTool_AskOpenAI_InvalidKey(t *testing.T) {
	// Save original key
	originalKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", originalKey)
	
	// Set invalid key
	os.Setenv("OPENAI_API_KEY", "invalid-key")
	
	tool := NewGetHelpTool("", "gpt-4o")
	ctx := context.Background()
	
	_, err := tool.askOpenAI(ctx, "test prompt")
	
	if err == nil {
		t.Error("Expected error with invalid API key")
	}
}

func TestMCPServer_HTTPHandler_BadJSON(t *testing.T) {
	server := NewMCPServer("test", "1.0.0")
	tool := NewGetHelpTool("", "gpt-4o")
	server.RegisterTool(tool)
	
	req := httptest.NewRequest(http.MethodPost, "/get_help", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()
	
	server.HandleHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestMCPServer_HTTPHandler_ValidRequest(t *testing.T) {
	server := NewMCPServer("test", "1.0.0")
	tool := NewGetHelpTool("", "gpt-4o")
	server.RegisterTool(tool)
	
	reqBody := `{
		"question": "How do I test this?",
		"summary": "Test project"
	}`
	
	req := httptest.NewRequest(http.MethodPost, "/get_help", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	
	server.HandleHTTP(w, req)
	
	// Should either succeed or fail gracefully
	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 200 or 503, got %d", w.Code)
	}
}

func TestMCPServer_ProcessRequest_Initialize(t *testing.T) {
	server := NewMCPServer("test", "1.0.0")
	
	req := JsonRPCRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{}`),
	}
	
	resp := server.ProcessRequest(req)
	
	if resp.Jsonrpc != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got %s", resp.Jsonrpc)
	}
	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %d", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
	
	result := resp.Result.(map[string]interface{})
	if result["protocolVersion"] != "2025-03-26" {
		t.Errorf("Expected protocolVersion '2025-03-26', got %v", result["protocolVersion"])
	}
}

func TestMCPServer_ProcessRequest_ToolsList(t *testing.T) {
	server := NewMCPServer("test", "1.0.0")
	tool := NewGetHelpTool("", "gpt-4o")
	server.RegisterTool(tool)
	
	req := JsonRPCRequest{
		Jsonrpc: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  json.RawMessage(`{}`),
	}
	
	resp := server.ProcessRequest(req)
	
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
	
	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]map[string]interface{})
	
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}
	
	if tools[0]["name"] != "get_help" {
		t.Errorf("Expected tool name 'get_help', got %v", tools[0]["name"])
	}
}