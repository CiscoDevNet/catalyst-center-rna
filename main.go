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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rna/pkg/archive"
	"rna/pkg/cli"
	"rna/pkg/logger"
	"rna/pkg/req"

	"github.com/rs/zerolog"
)

// Version comes from CL
var (
	log  zerolog.Logger
	args Args
)

func main() {
	log = logger.New()
	args = newArgs()

	cfg := cli.Config{
		Host:              args.Dnac,
		Username:          args.Username,
		Password:          args.Password,
		RetryDelay:        args.RetryDelay,
		RequestRetryCount: args.RequestRetryCount,
		BatchSize:         args.BatchSize,
	}

	// Initialize DNA HTTP client
	client, err := cli.GetClient(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing Catalyst Center client.")
	}

	// Create results archive
	arc, err := archive.NewWriter(args.Output)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error creating archive file: %s.", args.Output)
	}
	defer arc.Close()

	// Initiate requests
	reqs, err := req.GetRequests()
	if err != nil {
		log.Fatal().Err(err).Msgf("Error reading requests.")
	}

	// Use dependency-aware execution for optimal performance and correctness
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Starting optimized execution with %d total API requests\n", len(reqs))
	fmt.Println(strings.Repeat("=", 50))

	executor := cli.NewDependencyExecutor(client, arc, cfg, reqs)
	err = executor.Execute()
	if err != nil {
		log.Error().Err(err).Msg("Error during optimized execution.")
	}

	cli.CreateDummyFiles(arc)

	fmt.Println(strings.Repeat("=", 30))
	fmt.Println("Complete")
	fmt.Println(strings.Repeat("=", 30))

	path, err := os.Getwd()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot read current working directory")
	}
	outPath := filepath.Join(path, args.Output)

	if err != nil {
		log.Warn().Err(err).Msg("some data could not be fetched")
		log.Info().Err(err).Msgf("Available data written to %s.", outPath)
	} else {
		log.Info().Msg("Collection complete.")
		log.Info().Msgf("Please provide %s to Cisco Services for further analysis.", outPath)
	}
	cli.AddLogFile(arc)
}
