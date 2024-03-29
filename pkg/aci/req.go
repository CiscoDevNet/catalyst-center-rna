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
	"net/http"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Body wraps SJSON for building JSON body strings.
// Usage example:
//
//	Body{}.Set("fvTenant.attributes.name", "mytenant").Str
type Body struct {
	Str string
}

// Set sets a JSON path to a value.
func (body Body) Set(path, value string) Body {
	res, _ := sjson.Set(body.Str, path, value)
	body.Str = res
	return body
}

// SetRaw sets a JSON path to a raw string value.
// This is primarily used for building up nested structures, e.g.:
//
//	Body{}.SetRaw("fvTenant.attributes", Body{}.Set("name", "mytenant").Str).Str
func (body Body) SetRaw(path, rawValue string) Body {
	res, _ := sjson.SetRaw(body.Str, path, rawValue)
	body.Str = res
	return body
}

// Res creates a Res object, i.e. a GJSON result object.
func (body Body) Res() Res {
	return gjson.Parse(body.Str)
}

// Req wraps http.Request for API requests.
type Req struct {
	// HttpReq is the *http.Request obejct.
	HttpReq *http.Request
	// Refresh indicates whether token refresh should be checked for this request.
	// Pass NoRefresh to disable Refresh check.
	format  string
	Refresh bool
}

// NoRefresh prevents token refresh check.
// Primarily used by the Login and Refresh methods where this would be redundant.
func NoRefresh(req *Req) {
	req.Refresh = false
}

// Query sets an HTTP query parameter.
//
//	client.GetClass("fvBD", aci.Query("query-target-filter", `eq(fvBD.name,"bd-name")`))
//
// Or set multiple parameters:
//
//	client.GetClass("fvBD",
//	  aci.Query("rsp-subtree-include", "faults"),
//	  aci.Query("query-target-filter", `eq(fvBD.name,"bd-name")`))
func Query(k, v string) func(req *Req) {
	return func(req *Req) {
		q := req.HttpReq.URL.Query()
		q.Add(k, v)
		req.HttpReq.URL.RawQuery = q.Encode()
	}
}

func FileFormat(val string) func(req *Req) {
	return func(req *Req) {
		req.format = val
	}
}
