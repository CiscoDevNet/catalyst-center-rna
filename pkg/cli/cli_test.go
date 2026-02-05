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
