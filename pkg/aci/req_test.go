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

package aci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

// TestSetRaw tests the Body::SetRaw method.
func TestSetRaw(t *testing.T) {
	name := Body{}.SetRaw("a", `{"name":"a"}`).Res().Get("a.name").Str
	assert.Equal(t, "a", name)
}

// TestQuery tests the Query function.
func TestQuery(t *testing.T) {
	defer gock.Off()
	client := testClient()

	gock.New(testURL).Get("/url").MatchParam("foo", "bar").Reply(200)
	_, err := client.Get("/url", Query("foo", "bar"))
	assert.NoError(t, err)

	// Test case for comma-separated parameters
	gock.New(testURL).Get("/url").MatchParam("foo", "bar,baz").Reply(200)
	_, err = client.Get("/url", Query("foo", "bar,baz"))
	assert.NoError(t, err)
}
