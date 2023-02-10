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

package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestLogger(t *testing.T) {
	a := assert.New(t)

	fileBuf := &bytes.Buffer{}
	consoleBuf := &bytes.Buffer{}
	writer := MultiLevelWriter{
		file:    fileBuf,
		console: consoleBuf,
	}
	log := zerolog.New(writer).With().Timestamp().Logger()

	log.Debug().Msg("debug_test")
	log.Info().Msg("info_test")
	json := gjson.ParseBytes(fileBuf.Bytes())
	a.Equal("debug_test", json.Get(`..#(level="debug").message`).Str)
	a.Equal("info_test", json.Get(`..#(level="info").message`).Str)
	console := consoleBuf.String()
	a.True(strings.Contains(console, "info_test"))
	a.False(strings.Contains(console, "debug_test"))
}
