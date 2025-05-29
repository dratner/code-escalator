# MCP Escalator

A reusable Model Context Protocol (MCP) server framework with a built-in Claude-to-o3 escalation tool for routing difficult problems to OpenAI's advanced models.

## Installation

```bash
go install github.com/dratner/code-escalator@latest
```

Or clone and build locally:

```bash
git clone https://github.com/dratner/code-escalator
cd code-escalator
go build -o escalator
```

## Setup

### 1. Export OpenAI API Key

```bash
export OPENAI_API_KEY=your_openai_api_key_here
```

### 2. Running the Server

Start the MCP Escalator server:

```bash
./escalator
```

Or with custom options:

```bash
./escalator --port 9001 --summary ./PROJECT.md --model o3
```

### CLI Options

- `--summary`: Path to project summary file (default: ./README.md)
- `--port`: Port to listen on (default: 9001) 
- `--model`: OpenAI model to use (default: gpt-4o)
- `--sse`: Run as HTTP server instead of stdio mode (for testing)
- `-h`: Show help

## Registering with Claude Code

1. Build the binary:
   ```bash
   go build -o escalator
   ```

2. Copy the binary to your PATH with the correct name:
   ```bash
   sudo cp escalator /usr/local/bin/get_help
   ```

3. Register with Claude Code:
   ```bash
   claude mcp add get_help /usr/local/bin/get_help -t stdio
   ```

4. Optional: Configure with custom arguments:
   ```bash
   claude mcp add get_help "/usr/local/bin/get_help --summary ./PROJECT.md --model o1" -t stdio
   ```

5. The `get_help` tool will be available for escalating problems

## Using as MCP Framework

For other MCP implementations or frameworks, a manifest file (`get_help.json`) is included for reference. This follows the standard MCP server configuration format and can be adapted for non-Claude Code environments.

## Usage

### Sample Escalation Flow

Here's an example of how Claude Code would escalate a problem:

**User:** "How do I implement JWT authentication in Go?"

**Claude Code:** *tries to solve, encounters difficulty*

**Claude Code calls get_help:**
```json
{
  "question": "How do I implement JWT authentication in Go with proper security?",
  "summary": "This is a REST API project using Go with Gin framework for user management.",
  "relevant_code": "func authMiddleware() gin.HandlerFunc { ... }"
}
```

**MCP Escalator Response:**
```json
{
  "content": [
    {
      "type": "text",
      "text": "To implement JWT authentication in Go with proper security, you should:\n\n1. Use a proven JWT library like github.com/golang-jwt/jwt\n2. Store secrets securely using environment variables\n3. Implement proper token validation middleware...\n\n[detailed implementation guidance from OpenAI]"
    }
  ]
}
```

## Testing

Run unit tests:
```bash
go test -v
```

Run end-to-end live test (requires real OpenAI API key):
```bash
./scripts/e2e_live.sh
```

**Note:** The live test is marked as [manual] as it requires a valid OpenAI API key and will make real API calls.

## Architecture

The codebase is designed to be modular and reusable:

- **main.go** - Reusable MCP server framework with JSON-RPC 2.0 protocol handling
- **gethelp.go** - Specific tool implementation for OpenAI escalation
- **Tool interface** - Simple interface for adding new MCP tools

### Adding New Tools

To add a new MCP tool, implement the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() map[string]interface{}
    Call(arguments map[string]interface{}) ([]map[string]interface{}, error)
}
```

Example:

```go
// newtool.go
type MyTool struct {}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "Does something useful"
}

func (t *MyTool) Schema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "input": map[string]interface{}{
                "type": "string",
                "description": "Input parameter",
            },
        },
        "required": []string{"input"},
    }
}

func (t *MyTool) Call(args map[string]interface{}) ([]map[string]interface{}, error) {
    input := args["input"].(string)
    return []map[string]interface{}{
        {
            "type": "text",
            "text": "Processed: " + input,
        },
    }, nil
}

// Register in main.go:
server.RegisterTool(&MyTool{})
```

## HTTP API (Legacy)

For backward compatibility, an HTTP endpoint is available with `--sse` flag:

- **POST** `http://127.0.0.1:9001/get_help`

Request format:
```json
{
  "question": "string (required)",
  "summary": "string (required)", 
  "relevant_code": "string (optional)"
}
```

Response format (success):
```json
{
  "answer": "Response from OpenAI"
}
```

Response format (error):
```json
{
  "error": "The architect is currently unavailable. Please try again later."
}
```