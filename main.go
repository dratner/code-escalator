package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Tool interface for MCP tools
type Tool interface {
	Name() string
	Description() string
	Schema() map[string]interface{}
	Call(arguments map[string]interface{}) ([]map[string]interface{}, error)
}

// JSON-RPC structures
type JsonRPCRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JsonRPCResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// MCP Server
type MCPServer struct {
	tools      map[string]Tool
	serverInfo map[string]string
}

func NewMCPServer(name, version string) *MCPServer {
	return &MCPServer{
		tools: make(map[string]Tool),
		serverInfo: map[string]string{
			"name":    name,
			"version": version,
		},
	}
}

func (s *MCPServer) RegisterTool(tool Tool) {
	s.tools[tool.Name()] = tool
}

func (s *MCPServer) HandleInitialize() map[string]interface{} {
	return map[string]interface{}{
		"protocolVersion": "2025-03-26",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": s.serverInfo,
	}
}

func (s *MCPServer) HandleToolsList() map[string]interface{} {
	tools := make([]map[string]interface{}, 0, len(s.tools))
	
	for _, tool := range s.tools {
		tools = append(tools, map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"inputSchema": tool.Schema(),
		})
	}
	
	return map[string]interface{}{
		"tools": tools,
	}
}

func (s *MCPServer) HandleToolsCall(params json.RawMessage) (map[string]interface{}, map[string]interface{}) {
	var callParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	
	if err := json.Unmarshal(params, &callParams); err != nil {
		log.Printf("Failed to parse tools/call params: %v", err)
		return nil, map[string]interface{}{
			"code":    -32602,
			"message": "Invalid params",
		}
	}
	
	tool, exists := s.tools[callParams.Name]
	if !exists {
		log.Printf("Unknown tool: %s", callParams.Name)
		return nil, map[string]interface{}{
			"code":    -32602,
			"message": "Unknown tool",
		}
	}
	
	content, err := tool.Call(callParams.Arguments)
	if err != nil {
		log.Printf("Tool call failed: %v", err)
		return map[string]interface{}{
			"content": content,
			"isError": true,
		}, nil
	}
	
	return map[string]interface{}{
		"content": content,
	}, nil
}

func (s *MCPServer) ProcessRequest(req JsonRPCRequest) JsonRPCResponse {
	var resp JsonRPCResponse
	resp.Jsonrpc = "2.0"
	resp.ID = req.ID

	log.Printf("[%s] Got JSON-RPC request: method=%s, id=%d", time.Now().Format(time.RFC3339), req.Method, req.ID)

	switch req.Method {
	case "initialize":
		log.Println("Handling initialize")
		resp.Result = s.HandleInitialize()
	case "tools/list":
		log.Println("Handling tools/list")
		resp.Result = s.HandleToolsList()
	case "tools/call":
		log.Println("Handling tools/call")
		result, errorResp := s.HandleToolsCall(req.Params)
		if errorResp != nil {
			resp.Error = errorResp
		} else {
			resp.Result = result
		}
	default:
		log.Printf("Unknown method: %s", req.Method)
		resp.Error = map[string]interface{}{
			"code":    -32601,
			"message": "Method not found",
		}
	}

	return resp
}

func (s *MCPServer) RunStdio() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	
	for {
		var req JsonRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error decoding JSON-RPC: %v", err)
			continue
		}

		resp := s.ProcessRequest(req)
		
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Failed to write JSON-RPC response: %v", err)
		}
	}
}

// Legacy HTTP handler for backward compatibility
func (s *MCPServer) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Got HTTP request")

	if r.Method != http.MethodPost {
		log.Println("Called with a GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For HTTP mode, expect direct tool arguments
	var arguments map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&arguments); err != nil {
		log.Println("Couldn't decode the JSON")
		http.Error(w, "malformed request", http.StatusBadRequest)
		return
	}

	// Find the get_help tool (backward compatibility)
	tool, exists := s.tools["get_help"]
	if !exists {
		http.Error(w, `{"error":"Tool not available"}`, http.StatusServiceUnavailable)
		return
	}

	content, err := tool.Call(arguments)
	if err != nil {
		log.Printf("Tool call failed: %v", err)
		http.Error(w, `{"error":"The architect is currently unavailable. Please try again later."}`, http.StatusServiceUnavailable)
		return
	}

	// Return legacy format
	if len(content) > 0 && content[0]["type"] == "text" {
		response := map[string]string{"answer": content[0]["text"].(string)}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func init() {
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}
}

func main() {

	summaryFlag := flag.String("summary", "", "Path to project summary file (default: ./README.md)")
	portFlag := flag.Int("port", 9001, "Port to listen on")
	modelFlag := flag.String("model", "gpt-4o", "OpenAI model to use")
	sseFlag := flag.Bool("sse", false, "Run as HTTP server instead of stdio mode")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", "escalator")
		fmt.Fprintf(flag.CommandLine.Output(), "  MCP Escalator - Routes unsolved problems to OpenAI for clarification\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Create MCP server
	server := NewMCPServer("escalator", "1.0.0")
	
	// Register tools
	helpTool := NewGetHelpTool(*summaryFlag, *modelFlag)
	server.RegisterTool(helpTool)

	// Setup logging
	if !*sseFlag {
		logFile, err := os.OpenFile("/tmp/escalator.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
		}
	}

	if *sseFlag {
		// HTTP server mode
		log.Println("Starting HTTP server mode...")
		addr := fmt.Sprintf("127.0.0.1:%d", *portFlag)

		http.HandleFunc("/get_help", server.HandleHTTP)

		httpServer := &http.Server{
			Addr:         addr,
			ReadTimeout:  4 * time.Minute,
			WriteTimeout: 4 * time.Minute,
		}

		log.Printf("Starting MCP Escalator server on %s (summary: %s, model: %s)\n", addr, *summaryFlag, *modelFlag)
		log.Fatal(httpServer.ListenAndServe())
	} else {
		// stdio mode (default) - MCP protocol
		server.RunStdio()
	}
}