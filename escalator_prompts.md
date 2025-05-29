# Claude‑to‑o3 Escalator — Engineering Prompt Pack

Use each section below as a *stand‑alone* instruction to a coding LLM (e.g. Claude Code or GPT‑4o).  
All prompts assume a fresh Go 1.22 workspace on macOS with `OPENAI_API_KEY` set in the environment.

---

## Project Context  *(include in every chat)*

```
You are implementing “Claude‑to‑o3 Escalator”, a local tool that lets Claude Code escalate unsolved coding tasks
to OpenAI’s o3 model.  The workflow uses Anthropic’s **Model Context Protocol (MCP)** and an HTTP tool endpoint.

Key requirements
• MCP tool name: `get_help`
• Endpoint: http://127.0.0.1:9001/get_help  (plain HTTP, no TLS)
• End‑to‑end timeout: 4 minutes
• Project summary lives in ./README.md unless overridden by --summary flag
• Single live functional test must hit the real o3 model
```

---

## Prompt 1 — Create MCP Manifest

```
Create file manifest/get_help.json with:

{
  "name": "get_help",
  "description": "Routes unsolved problems to an external diagnostic agent (OpenAI o3) for clarification.",
  "url": "http://127.0.0.1:9001/get_help",
  "method": "POST",
  "input_schema": {
    "type": "object",
    "properties": {
      "title": { "type": "string" },
      "context": { "type": "string" },
      "last_attempt": { "type": "string" },
      "specific_question": { "type": "string" }
    },
    "required": ["title", "context", "last_attempt", "specific_question"]
  }
}

Validate against Anthropic’s MCP schema.
```

---

## Prompt 2 — Scaffold Go Project

```
Initialize a Go module named mcp‑escalator.

Directory layout:
  cmd/escalator/main.go
  internal/server/server.go
  internal/openai/client.go
  internal/prompt/builder.go
  manifest/get_help.json
  README.md
Ensure `go test ./...` passes (even if no tests yet).
```

---

## Prompt 3 — HTTP Listener

```
Implement internal/server with:

• Listen on 127.0.0.1:9001
• POST /get_help
• Read entire JSON body into struct {Title, Context, LastAttempt, SpecificQuestion string}
• ReadTimeout & WriteTimeout = 4 min
• On bad JSON → 400 with message "malformed request"

Unit test: POST invalid JSON returns 400.
```

---

## Prompt 4 — Project Summary Loader

```
Add --summary flag (default "") to cmd/escalator.
If empty, read ./README.md relative to CWD.
Expose func LoadSummary(path string) (string, error) in internal/prompt.
Unit test: loads file, errors when missing.
```

---

## Prompt 5 — OpenAI o3 Client

```
internal/openai/client.go:

• Require env OPENAI_API_KEY at init() else log.Fatal
• Func AskO3(ctx, prompt string) (string, error)
• Call POST https://api.openai.com/v1/chat/completions with model "o3"
• Content = prompt
• 3 retries on 429/5xx with 2 s, 4 s, 8 s backoff
Return assistant message content.
Unit test: table‑driven test with httptest.Server stubs.
```

---

## Prompt 6 — Prompt Builder

```
internal/prompt/builder.go:

Template:
  Here's help from the architect:

  <summary>

  ---
  **Problem Title:** <title>

  **Context:** <context>

  **Last Attempt:** <last_attempt>

  **Question:** <specific_question>

Replace placeholders; ensure resulting string ≤ 20 000 tokens (abort if exceeded).
Unit test: builder injects summary & fields correctly.
```

---

## Prompt 7 — Wire Handler to o3

```
In server handler:
• Build prompt via builder with summary text.
• Call openai.AskO3.
• On success → JSON {"answer": <o3‑content>} 200 OK.
• On failure → 503 {"error":"The architect is currently unavailable. Please try again later."}
Integration test with mocked client.
```

---

## Prompt 8 — End‑to‑End Live Test Script

```
Create scripts/e2e_live.sh:

1. Start server in background
2. curl -s -X POST http://127.0.0.1:9001/get_help       -d '{"title":"hello","context":"","last_attempt":"","specific_question":"Write a Go statement that prints hello world."}'       | jq -r .answer
3. Expect output contains 'fmt.Println("hello world")'
Exit non‑zero if not found.
Mark script as [manual] in README (requires live API key).
```

---

## Prompt 9 — CLI Flags & Help

```
cmd/escalator/main.go:

• Flags: --summary, --port (default 9001), --model (default o3)
• -h prints help with defaults.
Verify go vet & golint show no issues.
```

---

## Prompt 10 — Top‑Level README

```
Write README.md covering:
• Installation (go install)
• Exporting OPENAI_API_KEY
• Running server
• Registering manifest with Claude Code
• Sample escalation flow (copy‑paste transcript)
```

---

### End of Prompt Pack
