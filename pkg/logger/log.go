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
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/rs/zerolog"
)

const logFile = "rna_collector.log"

var logMux sync.Mutex

// Logger aliases the zerolog.Logger
type Logger = zerolog.Logger

var Log Logger

// MultiLevelWriter writes logs to file and console
type MultiLevelWriter struct {
	file    io.Writer
	console io.Writer
}

func (w MultiLevelWriter) Write(p []byte) (int, error) {
	logMux.Lock()
	count, err := w.file.Write(p)
	logMux.Unlock()
	return count, err
}

// WriteLevel writes log data for a given log level
func (w MultiLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if level >= zerolog.InfoLevel {
		n, err := w.console.Write(p)
		if err != nil {
			return n, err
		}
	}
	return w.file.Write(p)
}

// New creates a new logging instance.
func New() Logger {
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(fmt.Sprintf("cannot create log file %s", logFile))
	}

	// zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.DurationFieldInteger = true

	windows := false
	if runtime.GOOS == "windows" {
		windows = true
	}

	writer := MultiLevelWriter{
		file:    file,
		console: zerolog.ConsoleWriter{Out: os.Stderr, NoColor: windows},
	}
	return zerolog.New(writer).With().Timestamp().Logger()
}
