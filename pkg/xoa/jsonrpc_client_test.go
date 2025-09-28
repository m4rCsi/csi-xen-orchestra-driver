// Copyright 2025 Marc Siegenthaler
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xoa

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := ClientConfig{
		BaseURL: "https://xo.company.lan",
		Token:   "test-token",
		Timeout: 30 * time.Second,
	}

	client, err := NewJSONRPCClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client should not be nil")
	}

	if client.baseURL != config.BaseURL {
		t.Errorf("Expected baseURL %s, got %s", config.BaseURL, client.baseURL)
	}

	if client.token != config.Token {
		t.Errorf("Expected token %s, got %s", config.Token, client.token)
	}
}

func TestNewClientValidation(t *testing.T) {
	// Test missing BaseURL
	config := ClientConfig{
		Token: "test-token",
	}

	_, err := NewJSONRPCClient(config)
	if err == nil {
		t.Error("Expected error for missing BaseURL")
	}

	// Test missing Token
	config = ClientConfig{
		BaseURL: "https://xo.company.lan",
	}

	_, err = NewJSONRPCClient(config)
	if err == nil {
		t.Error("Expected error for missing Token")
	}
}

func TestGetWebSocketURL(t *testing.T) {
	client := &jsonRPCClient{
		baseURL: "https://xo.company.lan",
		token:   "test-token",
	}

	wsURL, err := client.getWebSocketURL()
	if err != nil {
		t.Fatalf("Failed to get WebSocket URL: %v", err)
	}

	expected := "wss://xo.company.lan/api/"
	if wsURL != expected {
		t.Errorf("Expected WebSocket URL %s, got %s", expected, wsURL)
	}
}

func TestGetWebSocketURLHTTP(t *testing.T) {
	client := &jsonRPCClient{
		baseURL: "http://xo.company.lan",
		token:   "test-token",
	}

	wsURL, err := client.getWebSocketURL()
	if err != nil {
		t.Fatalf("Failed to get WebSocket URL: %v", err)
	}

	expected := "ws://xo.company.lan/api/"
	if wsURL != expected {
		t.Errorf("Expected WebSocket URL %s, got %s", expected, wsURL)
	}
}
