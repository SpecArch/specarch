package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
)

// standardInput is what specarch mcp reads its client's messages from; the
// conformance suite gives a case's standard input here. No other command
// reads it.
var standardInput io.Reader = os.Stdin

// mcpRevision is the revision of the Model Context Protocol the server
// follows, and mcpRevisions every revision it answers initialize for, the
// newest first (ADR-214).
const mcpRevision = "2025-11-25"

var mcpRevisions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

// mcpRunJSON runs a command as --json does; set in init, since the
// commands table holds runMCP and runJSON reads that table.
var mcpRunJSON func(args []string, stdout, stderr io.Writer) int

func init() { mcpRunJSON = runJSON }

// mcpInstructions is what the server tells the client's model on
// initialize.
const mcpInstructions = "SpecArch is a compiler for specifications: it reads a specification written in YAML, lists every problem in it, and writes documents and code from it. Call the guide tool with agent true for the order of work, what to ask the owner and how to read a problem; validate after every edit to a specification. Paths are relative to the folder the server was started in. A tool that writes takes dry-run, which lists what it would write."

// JSON-RPC 2.0's error codes.
const (
	rpcParseError     = -32700
	rpcInvalidRequest = -32600
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// mcpTool is a tool as tools/list lists it: the keys of the protocol
// alone.
type mcpTool struct {
	Name        string                 `json:"name"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	InputSchema map[string]any         `json:"inputSchema"`
	Annotations capability.Annotations `json:"annotations"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpCallResult struct {
	Content           []mcpContent `json:"content"`
	StructuredContent any          `json:"structuredContent,omitempty"`
	IsError           bool         `json:"isError"`
}

// mcpServer holds the tools, each with the command it runs.
type mcpServer struct {
	tools    []capability.Tool
	commands map[string]capability.Command
}

// runMCP serves every command as a tool of the Model Context Protocol on
// the stdio transport (ADR-214), until standard input ends.
func runMCP(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintf(stderr, "specarch mcp takes no argument, given %s; an agent's client starts it as a stdio server\n\n%s", args[0], usage)
		return 2
	}
	commands, err := ownCommands()
	if err != nil {
		fmt.Fprintf(stderr, "specarch mcp: the command definitions built into this program cannot be read: %v\n", err)
		return 2
	}
	s := &mcpServer{commands: map[string]capability.Command{}}
	for _, t := range listing(commands, true).Tools {
		c, ok := toolCommand(t, commands)
		if !ok || c.Name == "mcp" {
			continue
		}
		s.tools = append(s.tools, t)
		s.commands[t.Name] = c
	}
	in := bufio.NewReader(standardInput)
	for {
		line, err := in.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			if answer := s.handle(line); answer != nil {
				data, merr := json.Marshal(answer)
				if merr != nil {
					data, _ = json.Marshal(rpcResponse{JSONRPC: "2.0", ID: answer.ID, Error: &rpcError{Code: -32603, Message: "cannot encode the answer as JSON: " + merr.Error()}})
				}
				if _, werr := stdout.Write(append(data, '\n')); werr != nil {
					fmt.Fprintf(stderr, "specarch mcp: cannot write to standard output: %v\n", werr)
					return 2
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return 0
		}
		if err != nil {
			fmt.Fprintf(stderr, "specarch mcp: cannot read standard input: %v\n", err)
			return 2
		}
	}
}

// toolCommand is the command a tool runs: the one whose words begin the
// tool's command, the longest such.
func toolCommand(t capability.Tool, commands []capability.Command) (capability.Command, bool) {
	var found capability.Command
	ok := false
	for _, c := range commands {
		w := c.Words()
		if len(w) <= len(t.Command)-1 && slices.Equal(w, t.Command[1:1+len(w)]) && (!ok || len(w) > len(found.Words())) {
			found, ok = c, true
		}
	}
	return found, ok
}

// handle answers one message, or nothing for a notification.
func (s *mcpServer) handle(line []byte) *rpcResponse {
	trimmed := bytes.TrimSpace(line)
	if !json.Valid(trimmed) {
		return &rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: rpcParseError, Message: "the line is not JSON; send one JSON-RPC 2.0 message per line"}}
	}
	var req rpcRequest
	if trimmed[0] != '{' || json.Unmarshal(trimmed, &req) != nil || req.JSONRPC != "2.0" || req.Method == "" {
		return &rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: rpcInvalidRequest, Message: "not a JSON-RPC 2.0 request: send one object per line with jsonrpc \"2.0\" and a method; batches are not taken"}}
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return nil // a notification is not answered
	}
	answer := func(result any) *rpcResponse { return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result} }
	fail := func(code int, message string) *rpcResponse {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: message}}
	}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		revision := mcpRevision
		if slices.Contains(mcpRevisions, p.ProtocolVersion) {
			revision = p.ProtocolVersion
		}
		return answer(map[string]any{
			"protocolVersion": revision,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "specarch", "title": "SpecArch", "version": specarch.Version},
			"instructions":    mcpInstructions,
		})
	case "ping":
		return answer(map[string]any{})
	case "tools/list":
		tools := make([]mcpTool, 0, len(s.tools))
		for _, t := range s.tools {
			tools = append(tools, mcpTool{Name: t.Name, Title: t.Title, Description: t.Description, InputSchema: t.InputSchema, Annotations: t.Annotations})
		}
		return answer(map[string]any{"tools": tools})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
			return fail(rpcInvalidParams, "tools/call needs params with the tool's name and its arguments as an object")
		}
		for _, t := range s.tools {
			if t.Name == p.Name {
				return answer(s.call(t, p.Arguments))
			}
		}
		names := make([]string, len(s.tools))
		for i, t := range s.tools {
			names[i] = t.Name
		}
		return fail(rpcInvalidParams, fmt.Sprintf("specarch has no tool %q; tools/list lists its tools: %s", p.Name, strings.Join(names, ", ")))
	}
	return fail(rpcMethodNotFound, fmt.Sprintf("specarch mcp has no method %q; it answers initialize, ping, tools/list and tools/call, as MCP revision %s", req.Method, mcpRevision))
}

// call runs a tool's command with --json and answers with what it printed.
func (s *mcpServer) call(t capability.Tool, input map[string]any) mcpCallResult {
	if input == nil {
		input = map[string]any{}
	}
	args, err := capability.CommandLine(t, s.commands[t.Name], input)
	if err != nil {
		return mcpCallResult{Content: []mcpContent{{Type: "text", Text: err.Error() + "; nothing was run"}}, IsError: true}
	}
	var out, errText bytes.Buffer
	status := mcpRunJSON(args, &out, &errText)
	r := mcpCallResult{Content: []mcpContent{{Type: "text", Text: strings.TrimRight(out.String(), "\n")}}, IsError: status == 2}
	var structured map[string]any
	if json.Unmarshal(out.Bytes(), &structured) == nil {
		r.StructuredContent = structured
	}
	if e := strings.TrimRight(errText.String(), "\n"); e != "" {
		r.Content = append(r.Content, mcpContent{Type: "text", Text: e})
	}
	return r
}
