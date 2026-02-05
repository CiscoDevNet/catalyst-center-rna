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
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
	//"github.com/tidwall/sjson"
)

var log zerolog.Logger

// ExecutionContext manages shared state in a thread-safe manner
type ExecutionContext struct {
	globalID     string
	version      int
	versionFound bool
	totalDevices int
	mu           sync.RWMutex
}

// NewExecutionContext creates a new context with default values
func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		version: 223, // Default version
	}
}

// Thread-safe getters and setters
func (ctx *ExecutionContext) SetGlobalID(id string) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.globalID = id
}

func (ctx *ExecutionContext) GetGlobalID() string {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.globalID
}

func (ctx *ExecutionContext) SetVersion(v int) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.version = v
	ctx.versionFound = true
}

func (ctx *ExecutionContext) GetVersion() (int, bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.version, ctx.versionFound
}

func (ctx *ExecutionContext) SetTotalDevices(count int) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.totalDevices = count
}

func (ctx *ExecutionContext) GetTotalDevices() int {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.totalDevices
}

// DependencyExecutor manages API execution with proper dependency handling
type DependencyExecutor struct {
	client aci.Client
	arc    archive.Writer
	cfg    Config
	ctx    *ExecutionContext

	// Request categories
	versionAPIs     []req.Request
	globalAPIs      []req.Request
	deviceCountAPI  *req.Request
	independentAPIs []req.Request
	dependentAPIs   []req.Request
}

// NewDependencyExecutor creates a new dependency-aware executor
func NewDependencyExecutor(client aci.Client, arc archive.Writer, cfg Config, requests []req.Request) *DependencyExecutor {
	executor := &DependencyExecutor{
		client: client,
		arc:    arc,
		cfg:    cfg,
		ctx:    NewExecutionContext(),
	}

	executor.categorizeRequests(requests)
	return executor
}

// categorizeRequests separates requests by their dependency requirements
func (e *DependencyExecutor) categorizeRequests(requests []req.Request) {
	for _, req := range requests {
		switch req.VarStore {
		case "version", "version2", "version3", "version4":
			e.versionAPIs = append(e.versionAPIs, req)
		case "globalId":
			e.globalAPIs = append(e.globalAPIs, req)
		case "deviceCount":
			e.deviceCountAPI = &req
		default:
			// Check if request depends on variables
			if len(req.Variable) > 0 {
				e.dependentAPIs = append(e.dependentAPIs, req)
			} else if req.Store {
				e.dependentAPIs = append(e.dependentAPIs, req)
			} else {
				e.independentAPIs = append(e.independentAPIs, req)
			}
		}
	}
}

// Execute runs all requests with proper dependency management
func (e *DependencyExecutor) Execute() error {
	log.Info().Msg("Starting dependency-aware execution...")

	// Phase 1: Version Detection (concurrent, first success wins)
	if err := e.executeVersionDetection(); err != nil {
		log.Warn().Err(err).Msg("Version detection failed, using default version 223")
	}

	// Phase 2: Critical Dependencies (sequential)
	if err := e.executeCriticalDependencies(); err != nil {
		return fmt.Errorf("critical dependencies failed: %w", err)
	}

	// Phase 3: Independent APIs (highly concurrent)
	if err := e.executeIndependentAPIs(); err != nil {
		log.Error().Err(err).Msg("Some independent APIs failed")
	}

	// Phase 4: Dependent APIs (sequential but version-filtered)
	if err := e.executeDependentAPIs(); err != nil {
		log.Error().Err(err).Msg("Some dependent APIs failed")
	}

	log.Info().Msg("Execution completed")
	return nil
}

// executeVersionDetection tries version detection APIs concurrently
func (e *DependencyExecutor) executeVersionDetection() error {
	if len(e.versionAPIs) == 0 {
		return nil
	}

	log.Info().Msgf("Detecting version using %d endpoints...", len(e.versionAPIs))

	versionChan := make(chan int, len(e.versionAPIs))
	var wg sync.WaitGroup

	for _, api := range e.versionAPIs {
		wg.Add(1)
		go func(req req.Request) {
			defer wg.Done()
			if err := FetchResource(e.client, req, e.arc, e.cfg, e.ctx); err != nil {
				log.Debug().Err(err).Msgf("Version detection failed for %s", req.Api)
				return
			}

			// Check if version was found
			if version, found := e.ctx.GetVersion(); found {
				select {
				case versionChan <- version:
					log.Info().Msgf("Version %d detected via %s", version, req.Api)
				default: // Channel full, another goroutine already succeeded
				}
			}
		}(api)
	}

	// Wait for first success or timeout
	go func() {
		wg.Wait()
		close(versionChan)
	}()

	select {
	case version := <-versionChan:
		log.Info().Msgf("Using detected version: %d", version)
		return nil
	case <-time.After(30 * time.Second):
		log.Warn().Msg("Version detection timeout, using default version 223")
		return nil
	}
}

// executeCriticalDependencies processes global ID and device count
func (e *DependencyExecutor) executeCriticalDependencies() error {
	// Process Global ID requests
	for _, req := range e.globalAPIs {
		if e.isVersionCompatible(req) {
			log.Info().Msgf("Fetching global ID via %s", req.Api)
			if err := FetchResource(e.client, req, e.arc, e.cfg, e.ctx); err != nil {
				log.Error().Err(err).Msgf("Failed to get global ID from %s", req.Api)
			}
		}
	}

	// Process Device Count request
	if e.deviceCountAPI != nil && e.isVersionCompatible(*e.deviceCountAPI) {
		log.Info().Msgf("Fetching device count via %s", e.deviceCountAPI.Api)
		if err := FetchResource(e.client, *e.deviceCountAPI, e.arc, e.cfg, e.ctx); err != nil {
			log.Error().Err(err).Msg("Failed to get device count")
		}
	}

	return nil
}

// executeIndependentAPIs processes all independent requests concurrently
func (e *DependencyExecutor) executeIndependentAPIs() error {
	// Filter by version compatibility
	compatibleReqs := e.filterByVersion(e.independentAPIs)

	if len(compatibleReqs) == 0 {
		log.Info().Msg("No independent APIs to execute")
		return nil
	}

	log.Info().Msgf("Executing %d independent APIs concurrently...", len(compatibleReqs))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, e.cfg.BatchSize)
	errorCount := 0
	var errorMux sync.Mutex

	for _, reqItem := range compatibleReqs {
		wg.Add(1)
		go func(r req.Request) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := FetchResource(e.client, r, e.arc, e.cfg, e.ctx); err != nil {
				errorMux.Lock()
				errorCount++
				errorMux.Unlock()
				log.Error().Err(err).Msgf("Independent API failed: %s", r.Prefix)
			}
		}(reqItem)
	}

	wg.Wait()

	if errorCount > 0 {
		log.Warn().Msgf("%d out of %d independent APIs failed", errorCount, len(compatibleReqs))
	} else {
		log.Info().Msg("All independent APIs completed successfully")
	}

	return nil
}

// executeDependentAPIs processes requests that depend on variables
func (e *DependencyExecutor) executeDependentAPIs() error {
	compatibleReqs := e.filterByVersion(e.dependentAPIs)

	if len(compatibleReqs) == 0 {
		log.Info().Msg("No dependent APIs to execute")
		return nil
	}

	log.Info().Msgf("Executing %d dependent APIs sequentially...", len(compatibleReqs))

	errorCount := 0
	for _, req := range compatibleReqs {
		if err := FetchResource(e.client, req, e.arc, e.cfg, e.ctx); err != nil {
			errorCount++
			log.Error().Err(err).Msgf("Dependent API failed: %s", req.Prefix)
		}
	}

	if errorCount > 0 {
		log.Warn().Msgf("%d out of %d dependent APIs failed", errorCount, len(compatibleReqs))
	} else {
		log.Info().Msg("All dependent APIs completed successfully")
	}

	return nil
}

// filterByVersion returns requests compatible with current version
func (e *DependencyExecutor) filterByVersion(requests []req.Request) []req.Request {
	version, _ := e.ctx.GetVersion()
	var filtered []req.Request

	for _, req := range requests {
		if e.isVersionCompatible(req) {
			filtered = append(filtered, req)
		} else {
			log.Debug().Msgf("Skipping %s due to version incompatibility (current: %d, required: %v)",
				req.Prefix, version, req.Version)
		}
	}

	return filtered
}

// isVersionCompatible checks if request is compatible with current version
func (e *DependencyExecutor) isVersionCompatible(req req.Request) bool {
	version, _ := e.ctx.GetVersion()
	return len(req.Version) == 0 || isVersionInList(version, req.Version)
}

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
	/*
		Function to determine if an API should be skipped based on the version
			negative (-) list version means that api will be ran for equal or lower version
			positive list version means that api will be ran for equal or higer versions
	*/
	for _, val := range list {
		if val < 0 {
			val = val * -1
			if version <= val {
				return true
			}
		} else {
			if version >= val {
				return true
			}
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
	fileContent := []byte(res.Raw)
	fileExtension := "json"
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

// handleConcurrentPagination fetches paginated data concurrently
func handleConcurrentPagination(client aci.Client, req req.Request, totalDevices int, mods []func(*aci.Req)) (gjson.Result, error) {
	const pageSize = 500
	pageCount := (totalDevices + pageSize - 1) / pageSize

	if pageCount <= 0 {
		return gjson.Result{}, fmt.Errorf("no pages to fetch (totalDevices: %d)", totalDevices)
	}

	log.Info().Msgf("Fetching %d pages concurrently for %s (total devices: %d)", pageCount, req.Prefix, totalDevices)

	type pageResult struct {
		page int
		data gjson.Result
		err  error
	}

	results := make(chan pageResult, pageCount)
	var wg sync.WaitGroup

	// Fetch all pages concurrently
	for page := 1; page <= pageCount; page++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			offset := (p-1)*pageSize + 1
			path := replacePathPlaceholder(req.Path, req.Variable, strconv.Itoa(offset))

			log.Debug().Msgf("Fetching page %d/%d (offset %d) for %s", p, pageCount, offset, req.Prefix)
			res, err := client.Get(path, mods...)
			results <- pageResult{page: p, data: res, err: err}
		}(page)
	}

	// Wait for all pages to complete
	wg.Wait()
	close(results)

	// Collect results in order
	pages := make([]gjson.Result, pageCount)
	for result := range results {
		if result.err != nil {
			return gjson.Result{}, fmt.Errorf("page %d failed: %w", result.page, result.err)
		}
		pages[result.page-1] = result.data
	}

	// Combine all pages into a single JSON array
	var combined strings.Builder
	combined.WriteString("[")

	for i, pageData := range pages {
		if i > 0 {
			combined.WriteString(",")
		}
		combined.WriteString(pageData.Raw)
	}

	combined.WriteString("]")

	log.Info().Msgf("Successfully combined %d pages for %s", pageCount, req.Prefix)
	return gjson.Parse(combined.String()), nil
}

// Fetch data via API.
func FetchResource(client aci.Client, req req.Request, arc archive.Writer, cfg Config, ctx *ExecutionContext) error {
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
		body = body.Set(k, v)
		mods = append(mods, aci.Query(k, v))
	}

	var err error
	var res gjson.Result

	// Determie if it's POST operation and version requirements
	currentVersion, _ := ctx.GetVersion()
	if req.Method == "POST" && (len(req.Version) == 0 || isVersionInList(currentVersion, req.Version)) {
		data = body.Str
		log.Info().Msgf("fetching %s...", req.Prefix)
		log.Debug().Str("url", req.Path).Msg("requesting resource")
		res, err = client.Post(req.Path, data, mods...)

	} else {
		// Check  version requirement to run API
		if len(req.Version) == 0 || isVersionInList(currentVersion, req.Version) {
			//Replace req.Path yaml variable with value
			if len(req.Variable) > 0 {
				trimmedVariable := strings.Trim(req.Variable, "{}")
				switch trimmedVariable {
				case "globalId":
					if globalID := ctx.GetGlobalID(); len(globalID) > 0 {
						req.Path = replacePathPlaceholder(req.Path, req.Variable, globalID)
					}
				case "offset":
					// Use concurrent pagination for massive performance improvement
					totalDevices := ctx.GetTotalDevices()
					if totalDevices <= 0 {
						log.Warn().Msgf("No devices found for pagination request: %s", req.Prefix)
						return nil
					}

					res, err = handleConcurrentPagination(client, req, totalDevices, mods)
					if err != nil {
						error := createErrorResult(err)
						writeToFileAndZip(filename, error, arc)
						log.Debug().
							TimeDiff("elapsed_time", time.Now(), startTime).
							Msgf("done: %s", req.Prefix)
						return fmt.Errorf("pagination failed for %s: %v", req.Path, err)
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
						ctx.SetGlobalID(res.Get("response.0.id").String())
					}
				case "version":
					if _, versionFound := ctx.GetVersion(); !versionFound {
						if res.Get("response.displayVersion").Exists() {
							stringVersion := res.Get("response.displayVersion").String()
							slicedVersion := stringVersion[0:7]
							newStringVersion := strings.ReplaceAll(slicedVersion, ".", "")
							if parsedVersion, err := strconv.Atoi(newStringVersion); err == nil {
								ctx.SetVersion(parsedVersion)
								log.Info().Msgf("version found is " + fmt.Sprint(parsedVersion))
							}
						}
					}
				case "version2":
					if _, versionFound := ctx.GetVersion(); !versionFound {
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
										ctx.SetVersion(223)
										log.Info().Msgf("version found is 223")
									}
									break
								}
							}
						}
					}
				case "version3":
					if _, versionFound := ctx.GetVersion(); !versionFound {
						if res.Get("response.version").Exists() {
							stringVersion := res.Get("response.version").String()
							slicedVersion := stringVersion[0:7]
							newStringVersion := strings.ReplaceAll(slicedVersion, ".", "")
							if parsedVersion, err := strconv.Atoi(newStringVersion); err == nil {
								ctx.SetVersion(parsedVersion)
								log.Info().Msgf("version found is " + fmt.Sprint(parsedVersion))
							}
						}
					}
				case "version4":
					if _, versionFound := ctx.GetVersion(); !versionFound {
						if res.Get("response.displayVersion").Exists() {
							stringVersion := res.Get("response.displayVersion").String()
							slicedVersion := stringVersion[0:7]
							newStringVersion := strings.ReplaceAll(slicedVersion, ".", "")
							if parsedVersion, err := strconv.Atoi(newStringVersion); err == nil {
								ctx.SetVersion(parsedVersion)
								log.Info().Msgf("version found is " + fmt.Sprint(parsedVersion))
							}
						}
					}
				case "deviceCount":
					if res.Get("response").Exists() {
						//Store device count value for future calls
						ctx.SetTotalDevices(int(res.Get("response").Int()))
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
