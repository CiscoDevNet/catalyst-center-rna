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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http" // AR ** looks unique to ACI
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// Client is an HTTP ACI API client.
// Use aci.NewClient to initiate a client.
// This will ensure proper cookie handling and processing of modifiers.
type Client struct {
	// HTTPClient is the *http.Client used for API requests.
	HTTPClient *http.Client
	// host is the DNAC IP or hostname, e.g. 10.0.0.1:80 (port is optional).
	host string
	// Usr is the DNAC username.
	Usr string
	// Pwd is the DNAC password.
	Pwd string
	// LastRefresh is the timestamp of the last token refresh interval.
	LastRefresh time.Time
	// Token is the current authentication token
	Token string
}

// NewClient creates a new ACI HTTP client.
// Pass modifiers in to modify the behavior of the client, e.g.
//  client, _ := NewClient("apic", "user", "password", RequestTimeout(120))
func NewClient(url, usr, pwd string, mods ...func(*Client)) (Client, error) {

	// Normalize the URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := http.Client{
		Timeout:   300 * time.Second,
		Transport: tr,
	}

	client := Client{
		HTTPClient: &httpClient,
		host:       url,
		Usr:        usr,
		Pwd:        pwd,
	}
	for _, mod := range mods {
		mod(&client)
	}
	return client, nil
}

// NewReq creates a new Req request for this client.
func (client Client) NewReq(method, uri string, body io.Reader, mods ...func(*Req)) Req {
	httpReq, err := http.NewRequest(method, client.host+":443"+uri, body)
	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("Accept", "application/json")
	httpReq.Header.Add("X-Auth-Token", client.Token)

	if err != nil {
		panic(err)
	}
	req := Req{
		HttpReq: httpReq,
		Refresh: true,
	}
	for _, mod := range mods {
		mod(&req)
	}
	return req
}

// RequestTimeout modifies the HTTP request timeout from the default of 60 seconds.
func RequestTimeout(x time.Duration) func(*Client) {
	return func(client *Client) {
		client.HTTPClient.Timeout = x * time.Second
	}
}

// Do makes a request.
// Requests for Do are built ouside of the client.
func (client *Client) Do(req Req) (Res, error) {
	httpRes, err := client.HTTPClient.Do(req.HttpReq)
	if err != nil {
		return Res{}, err
	}
	defer httpRes.Body.Close()

	body, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return Res{}, errors.New("cannot decode response body")
	}

	var res Res
	response := body
	if req.format == "html" {
		// encoding html into json
		// https://go.dev/blog/json
		type html struct {
			Content string
		}
		page := html{Content: string(body)}
		json_body, _ := json.Marshal(page)
		err1 := gjson.ValidBytes(json_body)
		if !err1 {
			return Res{}, errors.New("unable to save html")
		}
		response = json_body
	}
	res = Res(gjson.ParseBytes(response))

	if httpRes.StatusCode != http.StatusOK && httpRes.StatusCode != http.StatusAccepted {
		return Res{}, fmt.Errorf("received HTTP status %d", httpRes.StatusCode)
	}

	return res, nil
}

// Get makes a GET request and returns a GJSON result.
// Results will be the raw data structure as returned by the DNAC
func (client *Client) Get(path string, mods ...func(*Req)) (Res, error) {
	req := client.NewReq("GET", path, nil, mods...)
	res, err := client.Do(req)
	return res, err
}

// Post makes a POST request and returns a GJSON result.
// Hint: Use the Body struct to easily create POST body data.
func (client *Client) Post(path, data string, mods ...func(*Req)) (Res, error) {
	req := client.NewReq("POST", path, strings.NewReader(data), mods...)
	return client.Do(req)
}

// Post makes a POST request and returns a GJSON result.
// Hint: Use the Body struct to easily create POST body data.
func (client *Client) loginAuth(path, data string, mods ...func(*Req)) (Res, error) {
	req := client.getToken("POST", path, strings.NewReader(data), mods...)
	return client.Do(req)
}

// loginAuth creates the initial request to get the token used
// for subsecuent calls.
func (client Client) getToken(method, uri string, body io.Reader, mods ...func(*Req)) Req {
	httpReq, err := http.NewRequest(method, client.host+":443"+uri+"json", body)
	httpReq.Header.Add("Content-Type", "application/json")
	httpReq.Header.Add("Accept", "application/json")
	httpReq.SetBasicAuth(client.Usr, client.Pwd)

	if err != nil {
		panic(err)
	}
	req := Req{
		HttpReq: httpReq,
		Refresh: true,
	}
	for _, mod := range mods {
		mod(&req)
	}
	return req
}

// Login authenticates to the DNAC.
func (client *Client) Login() error {
	data := fmt.Sprintf(`{"username":"%s","password":"%s"}`,
		client.Usr,
		client.Pwd,
	)
	res, err := client.loginAuth("/dna/system/api/v1/auth/token", data, NoRefresh)
	if err != nil {
		return err
	}
	errText := res.Get("error").Str
	if errText != "" {
		return fmt.Errorf("authentication error: %s", errText)
	}
	client.Token = res.Get("Token").Str
	client.LastRefresh = time.Now()
	return nil
}
