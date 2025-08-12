package util

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	logger "github.com/wso2/open-mcp-auth-proxy/internal/logging"
)

type RPCEnvelope struct {
	Method string `json:"method"`
	Params any    `json:"params"`
	ID     any    `json:"id"`
}

// This function parses a JSON-RPC request from an HTTP request body
func ParseRPCRequest(r *http.Request) (*RPCEnvelope, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if len(bodyBytes) == 0 {
		return nil, nil
	}

	var env RPCEnvelope
	dec := json.NewDecoder(bytes.NewReader(bodyBytes))
	if err := dec.Decode(&env); err != nil && err != io.EOF {
		logger.Warn("Error parsing JSON-RPC envelope: %v", err)
		return nil, err
	}

	return &env, nil
}
