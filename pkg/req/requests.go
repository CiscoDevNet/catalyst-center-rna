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

// Package req contains the collector requests
package req

import (
	_ "embed"

	"gopkg.in/yaml.v2"
)

//go:embed reqs.yaml
var reqsData []byte

// Request is an HTTP request.
type Request struct {
	Api      string            // DNAC API
	Method   string            // Handle POST and GET
	Prefix   string            // Name for filename and class in DB
	Query    map[string]string // Query parameters
	Path     string            // URI
	File     string            // Optional name for files to store API results
	VarStore string            //Variable to store
	Variable string            //Variable to add to request before making call
	Store    bool              //Flag to determine if we need to store info from response
	Version  []int             // Useful to skip execution of APIs
}

// Normalize chooses correct class and path.
// This should be run on all manually created Requests structs.

func (req *Request) Normalize() *Request {
	if req.Prefix == "" {
		req.Prefix = req.Api
	}
	req.Path = req.Api
	return req
}

// GetRequests returns normalized requests
func GetRequests() (reqs []Request, err error) {
	err = yaml.Unmarshal(reqsData, &reqs)
	if err != nil {
		return
	}
	for i := 0; i < len(reqs); i++ {
		reqs[i].Normalize()
	}
	return
}
