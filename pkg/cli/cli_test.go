// Copyright (c) 2023 Cisco and/or its affiliates.

// This software is licensed to you under the terms of the Cisco Sample
// Code License, Version 1.1 (the "License"). You may obtain a copy of the
// License at

//                https://developer.cisco.com/docs/licenses

// All use of the material herein must be in accordance with the terms of
// the License. All rights not expressly granted by the License are
// reserved. Unless required by applicable law or agreed to separately in
// writing, software distributed under the License is distributed on an "AS
// IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied.

package cli

import (
	"testing"
	"time"

	"rna/pkg/aci"
	"rna/pkg/req"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"gopkg.in/h2non/gock.v1"
)

type mockArchiveWriter struct {
	files map[string][]byte
}

func (a mockArchiveWriter) Close() error {
	return nil
}

func (a mockArchiveWriter) Add(name string, content []byte) error {
	a.files[name] = content
	return nil
}

func TestFetch(t *testing.T) {
	a := assert.New(t)
	defer gock.Off()

	// Mock API
	gock.New("https://dnac").
		Get("/api/test.json").
		Reply(200).
		BodyString(aci.Body{}.
			Set("response.0.fqn", "ndp-platform:1.6.1715").
			Set("response.1.name", "ndp-platform").
			Str)

	// Test client
	client, _ := aci.NewClient("dnac", "usr", "pwd")
	client.LastRefresh = time.Now()
	gock.InterceptClient(client.HTTPClient)

	// Test request
	req := req.Request{
		Path: "/api/test.json",
		File: "api_test",
	}

	// Mock archive
	arc := mockArchiveWriter{
		files: make(map[string][]byte),
	}

	// Create execution context for the test
	ctx := NewExecutionContext()

	// Write zip
	err := FetchResource(client, req, arc, NewConfig(), ctx)
	a.NoError(err)

	// Verify content written to mock archive
	content, ok := arc.files["api_test.json"]
	a.True(ok)
	api := gjson.ParseBytes(content).Get("response")
	a.Equal("ndp-platform:1.6.1715", api.Get("0.fqn").Str)
	a.Equal("ndp-platform", api.Get("1.name").Str)
}

func TestIsVersionInList(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		constraints []string
		mode        string
		expected    bool
	}{
		{name: "Lower or equal match", version: "2.3.7.7", constraints: []string{"<=2.3.7.7"}, mode: "", expected: true},
		{name: "Lower or equal no match", version: "2.3.7.10", constraints: []string{"<=2.3.7.7"}, mode: "", expected: false},
		{name: "Higher or equal match", version: "2.3.7.10", constraints: []string{">=2.3.7.9"}, mode: "", expected: true},
		{name: "3.x is higher than 2.x", version: "3.1.1", constraints: []string{">=2.3.7.10"}, mode: "", expected: true},
		{name: "Exact match", version: "3.1.1", constraints: []string{"=3.1.1"}, mode: "", expected: true},
		{name: "OR constraints", version: "2.2.3", constraints: []string{"<=2.2.3", ">=3.0.0"}, mode: "", expected: true},
		{name: "AND constraints in range", version: "2.3.7.10", constraints: []string{">=2.3.7.9", "<3.1.6"}, mode: "and", expected: true},
		{name: "AND constraints out of range", version: "3.1.6", constraints: []string{">=2.3.7.9", "<3.1.6"}, mode: "and", expected: false},
		{name: "Unknown mode falls back to OR", version: "3.1.6", constraints: []string{">=2.3.7.9", "<3.1.6"}, mode: "unexpected", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isVersionInList(tt.version, tt.constraints, tt.mode))
		})
	}
}

func TestParseDetectedVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "Four segments", input: "2.3.7.10", expected: "2.3.7.10", wantErr: false},
		{name: "Three segments", input: "3.1.1", expected: "3.1.1", wantErr: false},
		{name: "With suffix", input: "2.3.7.10-build45", expected: "2.3.7.10", wantErr: false},
		{name: "Invalid", input: "version-unknown", expected: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDetectedVersion(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCategorizeRequests_DeviceCountPointerUsesCorrectRequest(t *testing.T) {
	a := assert.New(t)

	requests := []req.Request{
		{Api: "/dna/intent/api/v1/network-device/count", Path: "/dna/intent/api/v1/network-device/count", VarStore: "deviceCount"},
		{Api: "/api/v1/registration/cdnaproxy/assembler-data?deviceType=DNAC", Path: "/api/v1/registration/cdnaproxy/assembler-data?deviceType=DNAC"},
	}

	executor := NewDependencyExecutor(aci.Client{}, mockArchiveWriter{files: make(map[string][]byte)}, NewConfig(), requests)

	if a.NotNil(executor.deviceCountAPI) {
		a.Equal("/dna/intent/api/v1/network-device/count", executor.deviceCountAPI.Api)
		a.Equal("deviceCount", executor.deviceCountAPI.VarStore)
	}
}
