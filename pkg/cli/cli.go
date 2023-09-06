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
	//"github.com/tidwall/sjson"
)

var log zerolog.Logger
var globalId string
var version = 223
var versionFound bool

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
		return aci.Client{}, fmt.Errorf("failed to create Catalyst Center client: %v", err)
	}

	// Authenticate
	log.Info().Str("host", cfg.Host).Msg("Catalyst Center host")
	log.Info().Str("user", cfg.Username).Msg("Catalyst Center username")
	log.Info().Msg("Authenticating to Catalyst Center...")
	if err := client.Login(); err != nil {
		return aci.Client{}, fmt.Errorf("cannot authenticate to Catalyst Center at %s: %v", cfg.Host, err)
	}
	log.Info().Msg("Authentication successfull")
	return client, nil
}

func replacePathPlaceholder(path string, valueToRaplace, replacement string) string {
	tempPath := strings.Replace(path, valueToRaplace, replacement, 1)
	return tempPath
}

func isVersionInList(version int, list []int) bool {
	for _, val := range list {
		if val == version {
			return true
		}
	}
	return false
}

func setFileName(req req.Request) string {
	var fileName string
	if len(req.File) > 0 {
		fileName = req.File
	} else {
		fileName = strings.ReplaceAll(req.Prefix, "/", "_")
	}
	return fileName
}

func writeToFileAndZip(fileName string, res gjson.Result, arc archive.Writer) error {
	// save APIs call to file
	var fileContent []byte
	fileExtension := "json"
	fileContent = []byte(res.Raw)
	err := arc.Add(fileName+"."+fileExtension, fileContent)
	if err != nil {
		return err
	}
	return nil
}

func createErrorResult(err error) gjson.Result {
	// Create a custom gjson.Result representing the error
	// For example, you can use a JSON string to represent the error
	// In this example, we will use a simple message as the error
	errorJSON := fmt.Sprintf(`{"error" : "%s"}`, err.Error())

	// Parse the JSON string to create the gjson.Result
	errorResult := gjson.Parse(errorJSON)

	return errorResult
}

// Fetch data via API.
func FetchResource(client aci.Client, req req.Request, arc archive.Writer, cfg Config) error {
	var data string = ""
	body := aci.Body{Str: data}
	log = logger.New()
	startTime := time.Now()
	log.Debug().Time("start_time", startTime).Msgf("begin: %s", req.Prefix)
	var mods []func(*aci.Req)
	filename := setFileName(req)

	//Assign values to dynamic queries
	for k, v := range req.Query {
		switch k {
		case "startTime":
			//today - 1 day and 1 hour
			start := time.Now().UnixMilli() - 950000
			v = strconv.Itoa(int(start))
		case "endTime":
			//today - 1 hour
			end := time.Now().UnixMilli() - 600000
			v = strconv.Itoa(int(end))
		case "toDate":
			// epoch time yesterday
			toDate := time.Now().AddDate(0, 0, -1)
			y, m, d := toDate.Date()
			output := fmt.Sprintf("%d-%d-%d", y, int(m), d)
			v = output
		case "fromDate":
			//epoch time yesterday last year
			now := time.Now()
			startDate := now.AddDate(-1, 0, -1)
			y, m, d := startDate.Date()
			output := fmt.Sprintf("%d-%d-%d", y, int(m), d)
			v = output
		}
		path := fmt.Sprintf("%s", k)
		body = body.Set(path, v)
		mods = append(mods, aci.Query(k, v))
	}

	var err error
	var res gjson.Result

	// Determie if it's POST operation and version requirements
	if req.Method == "POST" && (len(req.Version) == 0 || isVersionInList(version, req.Version)) {
		data = body.Str
		log.Info().Msgf("fetching %s...", req.Prefix)
		log.Debug().Str("url", req.Path).Msg("requesting resource")
		res, err = client.Post(req.Path, data, mods...)

	} else {
		// Check  version requirement to run API
		if len(req.Version) == 0 || isVersionInList(version, req.Version) {
			//Replace req.Path yaml variable with value
			if len(req.Variable) > 0 {
				trimmedVariable := strings.Trim(req.Variable, "{}")
				switch trimmedVariable {
				case "globalId":
					if len(globalId) > 0 {
						req.Path = replacePathPlaceholder(req.Path, req.Variable, globalId)
					}
				}
			}

			log.Info().Msgf("fetching %s...", req.Prefix)
			log.Debug().Str("url", req.Path).Msg("requesting resource")
			res, err = client.Get(req.Path, mods...)

			//Check if we need to store something from response
			if req.Store {
				switch req.VarStore {
				case "globalId":
					if res.Get("response.0.id").Exists() {
						//Store GlobalId value for future calls
						globalId = res.Get("response.0.id").String()
					}
				case "version":
					if versionFound == false {
						if res.Get("response.displayVersion").Exists() {
							stringVersion := res.Get("response.displayVersion").String()
							slicedVersion := stringVersion[0:5]
							newStringVersion := strings.ReplaceAll(slicedVersion, ".", "")
							version, err = strconv.Atoi(newStringVersion)
							versionFound = true
							log.Info().Msgf("version found is " + fmt.Sprint(version))
						}
					}
				case "version2":
					if versionFound == false {
						dictArray := res.Get("response").Array()
						var versionValue, trimmedVersion string
						for _, elem := range dictArray {
							appstacks := elem.Get("appstacks").Array()
							for _, appstack := range appstacks {
								appstackValue := appstack.String()
								if strings.Contains(appstackValue, "maglev-system") {
									versionValue = strings.Split(appstackValue, ":")[1]
									trimmedVersion = versionValue[0:3]
									if trimmedVersion == "1.6" {
										// if 1.6  set version  to 223
										version = 223
										versionFound = true
										log.Info().Msgf("version found is " + fmt.Sprint(version))
									}
									break
								}
							}
						}
					}
				case "version3":
					if versionFound == false {
						if res.Get("response.version").Exists() {
							stringVersion := res.Get("response.version").String()
							slicedVersion := stringVersion[0:5]
							newStringVersion := strings.ReplaceAll(slicedVersion, ".", "")
							version, err = strconv.Atoi(newStringVersion)
							versionFound = true
							log.Info().Msgf("version found is " + fmt.Sprint(version))
						}
					}
				}
			}
		}
	}

	//Retry for requestRetryCount times
	for retries := 0; err != nil && retries < cfg.RequestRetryCount; retries++ {
		log.Warn().Err(err).Msgf("request failed for %s. Retrying after %d seconds.",
			req.Path, cfg.RetryDelay)
		time.Sleep(time.Second * time.Duration(cfg.RetryDelay))
		res, err = client.Get(req.Path, mods...)
	}

	if err != nil {
		error := createErrorResult(err)
		writeToFileAndZip(filename, error, arc)
		log.Debug().
			TimeDiff("elapsed_time", time.Now(), startTime).
			Msgf("done: %s", req.Prefix)
		return fmt.Errorf("request failed for %s: %v", req.Path, err)
	}
	if res.Type != gjson.Null {
		writeToFileAndZip(filename, res, arc)
		log.Debug().
			TimeDiff("elapsed_time", time.Now(), startTime).
			Msgf("done: %s", req.Prefix)
		log.Info().Msgf("%s > Complete", req.Prefix)
	}
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
