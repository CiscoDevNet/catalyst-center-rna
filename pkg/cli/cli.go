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
	"fmt"
	"os"
	"rna/pkg/aci"
	"rna/pkg/archive"
	"rna/pkg/logger"
	"rna/pkg/req"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
)

var log zerolog.Logger

// Config is CLI conifg
type Config struct {
	Host              string
	Username          string
	Password          string
	RetryDelay        int
	RequestRetryCount int
	BatchSize         int
}

// NewConfig populates default values
func NewConfig() Config {
	return Config{
		RequestRetryCount: 2,
		RetryDelay:        10,
		BatchSize:         10,
	}
}

func GetClient(cfg Config) (aci.Client, error) {
	log = logger.New()
	client, err := aci.NewClient(
		cfg.Host, cfg.Username, cfg.Password,
		aci.RequestTimeout(600),
	)
	if err != nil {
		return aci.Client{}, fmt.Errorf("failed to create DNAC client: %v", err)
	}

	// Authenticate
	log.Info().Str("host", cfg.Host).Msg("DNAC host")
	log.Info().Str("user", cfg.Username).Msg("DNAC username")
	log.Info().Msg("Authenticating to the DNAC...")
	if err := client.Login(); err != nil {
		return aci.Client{}, fmt.Errorf("cannot authenticate to the DNAC at %s: %v", cfg.Host, err)
	}
	log.Info().Msg("Authentication successfull")
	return client, nil
}

// Fetch data via API.
func FetchResource(client aci.Client, req req.Request, arc archive.Writer, cfg Config) error {
	var data string = ""
	var resplaceholder gjson.Result
	log = logger.New()
	startTime := time.Now()
	log.Debug().Time("start_time", startTime).Msgf("begin: %s", req.Prefix)

	log.Info().Msgf("fetching %s...", req.Prefix)
	log.Debug().Str("url", req.Path).Msg("requesting resource")

	var mods []func(*aci.Req)

	htmlFormat := false
	if len(req.Format) > 0 {
		mods = append(mods, aci.FileFormat(req.Format))
		htmlFormat = true
	}

	// var mods []func(*aci.Req)
	//query := url.Values{}

	for k, v := range req.Query {
		if k == "startTime" {
			start := time.Now().UnixMilli() - 950000
			v = strconv.Itoa(int(start))
		} else if k == "endTime" {
			end := time.Now().UnixMilli() - 600000
			v = strconv.Itoa(int(end))
		} else if k == "toDate" {
			toDate := time.Now().AddDate(0, 0, -1)
			y, m, d := toDate.Date()
			output := fmt.Sprintf("%d-%d-%d", y, int(m), d)
			v = output
		} else if k == "fromDate" {
			now := time.Now()
			startDate := now.AddDate(-1, 0, -1)
			y, m, d := startDate.Date()
			output := fmt.Sprintf("%d-%d-%d", y, int(m), d)
			v = output
		}
		mods = append(mods, aci.Query(k, v))
	}

	if req.Method == "POST" {
		res, err := client.Post(req.Path, data, mods...)
		resplaceholder = res
		for retries := 0; err != nil && retries < cfg.RequestRetryCount; retries++ {
			log.Warn().Err(err).Msgf("request failed for %s. Retrying after %d seconds.",
				req.Path, cfg.RetryDelay)
			time.Sleep(time.Second * time.Duration(cfg.RetryDelay))
			res, err = client.Post(req.Path, data, mods...)
			resplaceholder = res
		}
	} else {
		//Handle tenants individually for scale purposes
		res, err := client.Get(req.Path, mods...)
		resplaceholder = res
		//Retry for requestRetryCount times
		for retries := 0; err != nil && retries < cfg.RequestRetryCount; retries++ {
			log.Warn().Err(err).Msgf("request failed for %s. Retrying after %d seconds.",
				req.Path, cfg.RetryDelay)
			time.Sleep(time.Second * time.Duration(cfg.RetryDelay))
			res, err = client.Get(req.Path, mods...)
			resplaceholder = res
		}

		if err != nil {
			return fmt.Errorf("request failed for %s: %v", req.Path, err)
		}
		log.Info().Msgf("%s > Complete", req.Prefix)
	}

	var fileName string
	if len(req.File) > 0 {
		fileName = req.File
	} else {
		fileName = strings.ReplaceAll(req.Prefix, "/", "_")
	}

	// save APIs call to file
	var fileContent []byte
	fileExtension := "json"
	if htmlFormat {
		stringTest := gjson.Get(resplaceholder.Raw, "Content")
		fileContent = []byte(stringTest.Str)
		fileExtension = "html.txt"
	} else {
		fileContent = []byte(resplaceholder.Raw)
	}

	err := arc.Add(fileName+"."+fileExtension, fileContent)
	if err != nil {
		return err
	}

	log.Debug().
		TimeDiff("elapsed_time", time.Now(), startTime).
		Msgf("done: %s", req.Prefix)
	return nil
}

func CreateDummyFiles(arc archive.Writer) error {
	dummyFiles := []string{"dnac_api_diagnostic_bundle", "css_dnac_healthcheck_collector_api"}

	for _, name := range dummyFiles {
		err := arc.Add(name, []byte(""))
		if err != nil {
			return err
		}
	}
	return nil
}

// Add rna_collector.log to zip file
func AddLogFile(arc archive.Writer) error {
	logFile := "rna_collector.log"
	fileContent, err := os.ReadFile(logFile)
	if err != nil {
		panic(err)
	}
	err2 := arc.Add(logFile, fileContent)
	if err2 != nil {
		return err
	}

	return nil
}
