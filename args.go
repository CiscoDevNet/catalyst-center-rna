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
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/alexflint/go-arg"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh/terminal"
)

const resultZip = "dnac_rna_hc_collection.zip"

// input collects CLI input.
func input(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s ", prompt)
	input, _ := reader.ReadString('\n')
	return strings.Trim(input, "\r\n")
}

// Args are command line parameters.
type Args struct {
	Dnac              string `arg:"-a,--address" help:"Catalyst Center hostname or IP address"`
	Username          string `arg:"-u,--username" help:"Catalyst Center username"`
	Password          string `arg:"-p,--password" help:"Catalyst Center password"`
	Output            string `arg:"-o,--output" help:"Output file"`
	RequestRetryCount int    `arg:"--request-retry-count" default:"3" help:"Times to retry a failed request"`
	RetryDelay        int    `arg:"--retry-delay" default:"10" help:"Seconds to wait before retry"`
	BatchSize         int    `arg:"--batch-size" default:"10" help:"Max request to send in parallel"`
}

// Description is the CLI description string.
func (Args) Description() string {
	return "RNA - Rapid Network Assesment for Catalyst Center."
}

// Version is the CLI version string.
func (Args) Version() string {
	return fmt.Sprintf("version %s", Version)
}

func isEnvExist(key string) bool {
	if _, ok := os.LookupEnv(key); ok {
		return true
	}
	return false
}

// NewArgs collects the CLI args and creates a new 'Args'.
func newArgs() Args {
	args := Args{Output: resultZip}
	arg.MustParse(&args)

	if _, err := os.Stat(".env"); err == nil {
		godotenv.Load(".env")
		if isEnvExist("DNAC_IP") {
			args.Dnac = os.Getenv("DNAC_IP")
		}
		if isEnvExist("DNAC_USER") {
			args.Username = os.Getenv("DNAC_USER")
		}
		if isEnvExist("PASSWORD") {
			args.Password = os.Getenv("PASSWORD")
		}
	} else {
		if args.Dnac == "" {
			args.Dnac = input("Catalyst Center IP:")
		}
		if args.Username == "" {
			args.Username = input("Username:")
		}
		if args.Password == "" {
			fmt.Print("Password: ")
			pwd, _ := terminal.ReadPassword(int(syscall.Stdin))
			args.Password = string(pwd)
			fmt.Println()
		}
	}
	return args
}
