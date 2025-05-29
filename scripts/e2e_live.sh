#!/bin/bash

# End-to-end live test for MCP Escalator
# This test requires a real OPENAI_API_KEY

set -e

echo "Starting MCP Escalator server..."

# Start server in background
go run main.go &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Function to cleanup on exit
cleanup() {
    echo "Stopping server..."
    kill $SERVER_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "Testing live OpenAI o3 integration..."

# Make request to the server
RESPONSE=$(curl -s -X POST http://127.0.0.1:9001/get_help \
    -H "Content-Type: application/json" \
    -d '{
        "question": "Write a Go statement that prints hello world.",
        "summary": "This is a test project for demonstrating MCP escalation to OpenAI o3."
    }')

echo "Response: $RESPONSE"

# Extract answer using jq
ANSWER=$(echo "$RESPONSE" | jq -r '.answer // .error // empty')

if [ -z "$ANSWER" ]; then
    echo "FAIL: No answer or error returned"
    exit 1
fi

# Check if response contains expected content
if echo "$ANSWER" | grep -q 'fmt.Println("hello world")'; then
    echo "PASS: Found expected Go print statement"
    exit 0
elif echo "$ANSWER" | grep -q 'fmt.Print'; then
    echo "PASS: Found Go print function (close enough)"
    exit 0
elif echo "$ANSWER" | grep -q 'hello world'; then
    echo "PASS: Response contains 'hello world'"
    exit 0
else
    echo "FAIL: Expected output containing 'fmt.Println(\"hello world\")' not found"
    echo "Got: $ANSWER"
    exit 1
fi