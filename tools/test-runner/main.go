package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Comprehensive test runner for TFUG
// Runs multiple test scenarios and generates coverage reports

type TestSuite struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Tests       []TestCase  `json:"tests"`
	Setup       []string    `json:"setup,omitempty"`
	Teardown    []string    `json:"teardown,omitempty"`
}

type TestCase struct {
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	ConfigDir      string            `json:"config_dir"`
	RulesFile      string            `json:"rules_file"`
	Command        []string          `json:"command"`
	ExpectedExit   int               `json:"expected_exit"`
	ExpectedOutput ExpectedOutput    `json:"expected_output"`
	Timeout        string            `json:"timeout,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
}

type ExpectedOutput struct {
	ContainsAll []string `json:"contains_all,omitempty"`
	ContainsAny []string `json:"contains_any,omitempty"`
	NotContains []string `json:"not_contains,omitempty"`
	JSONPath    []JSONAssert `json:"json_path,omitempty"`
	Regex       []string     `json:"regex,omitempty"`
}

type JSONAssert struct {
	Path     string      `json:"path"`
	Expected interface{} `json:"expected"`
	Type     string      `json:"type,omitempty"` // "equals", "greater", "contains"
}

type TestResult struct {
	Name       string        `json:"name"`
	Passed     bool          `json:"passed"`
	Duration   time.Duration `json:"duration"`
	Output     string        `json:"output"`
	Error      string        `json:"error,omitempty"`
	ExitCode   int           `json:"exit_code"`
}

type SuiteResult struct {
	Suite     string       `json:"suite"`
	Passed    bool         `json:"passed"`
	Total     int          `json:"total"`
	Failed    int          `json:"failed"`
	Duration  time.Duration `json:"duration"`
	Results   []TestResult `json:"results"`
}

func main() {
	var (
		suiteFile = flag.String("suite", "", "Test suite JSON file")
		pattern   = flag.String("pattern", "", "Run tests matching pattern")
		verbose   = flag.Bool("verbose", false, "Verbose output")
		parallel  = flag.Int("parallel", 1, "Number of parallel test processes")
		timeout   = flag.String("timeout", "5m", "Default timeout per test")
		format    = flag.String("format", "text", "Output format: text, json, junit")
		outputDir = flag.String("output-dir", "test-results", "Output directory for reports")
	)
	flag.Parse()

	if *suiteFile == "" {
		log.Fatal("Please specify a test suite file with -suite")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Load test suite
	suite, err := loadTestSuite(*suiteFile)
	if err != nil {
		log.Fatalf("Failed to load test suite: %v", err)
	}

	// Filter tests if pattern provided
	if *pattern != "" {
		suite.Tests = filterTests(suite.Tests, *pattern)
	}

	fmt.Printf("Running TFUG Test Suite: %s\n", suite.Name)
	fmt.Printf("Description: %s\n", suite.Description)
	fmt.Printf("🧪 Tests: %d\n\n", len(suite.Tests))

	// Run setup commands
	if len(suite.Setup) > 0 {
		fmt.Println("Running setup commands...")
		for _, cmd := range suite.Setup {
			if err := runCommand(cmd, "", 30*time.Second); err != nil {
				log.Fatalf("Setup failed: %v", err)
			}
		}
		fmt.Println()
	}

	// Run tests
	startTime := time.Now()
	results := runTests(suite.Tests, TestOptions{
		Parallel:       *parallel,
		DefaultTimeout: parseDuration(*timeout),
		Verbose:        *verbose,
	})
	duration := time.Since(startTime)

	// Count results
	passed := 0
	for _, result := range results {
		if result.Passed {
			passed++
		}
	}

	suiteResult := SuiteResult{
		Suite:    suite.Name,
		Passed:   passed == len(results),
		Total:    len(results),
		Failed:   len(results) - passed,
		Duration: duration,
		Results:  results,
	}

	// Run teardown commands
	if len(suite.Teardown) > 0 {
		fmt.Println("\n🧹 Running teardown commands...")
		for _, cmd := range suite.Teardown {
			if err := runCommand(cmd, "", 30*time.Second); err != nil {
				fmt.Printf("Teardown warning: %v\n", err)
			}
		}
	}

	// Generate reports
	switch *format {
	case "json":
		generateJSONReport(suiteResult, filepath.Join(*outputDir, "results.json"))
	case "junit":
		generateJUnitReport(suiteResult, filepath.Join(*outputDir, "junit.xml"))
	case "text":
		printTextReport(suiteResult, *verbose)
	default:
		log.Fatalf("Unknown output format: %s", *format)
	}

	// Save detailed results
	generateJSONReport(suiteResult, filepath.Join(*outputDir, "detailed-results.json"))

	if !suiteResult.Passed {
		os.Exit(1)
	}
}

type TestOptions struct {
	Parallel       int
	DefaultTimeout time.Duration
	Verbose        bool
}

func loadTestSuite(filename string) (*TestSuite, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var suite TestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return nil, err
	}

	return &suite, nil
}

func filterTests(tests []TestCase, pattern string) []TestCase {
	var filtered []TestCase
	for _, test := range tests {
		if strings.Contains(test.Name, pattern) || strings.Contains(test.Description, pattern) {
			filtered = append(filtered, test)
		}
	}
	return filtered
}

func runTests(tests []TestCase, opts TestOptions) []TestResult {
	results := make([]TestResult, len(tests))
	
	if opts.Parallel <= 1 {
		// Sequential execution
		for i, test := range tests {
			results[i] = runTest(test, opts)
		}
	} else {
		// Parallel execution
		ch := make(chan struct {
			index int
			result TestResult
		}, opts.Parallel)
		
		// Start workers
		for i := 0; i < opts.Parallel; i++ {
			go func() {
				for j := range make(chan int) {
					if j >= len(tests) {
						return
					}
					ch <- struct {
						index int
						result TestResult
					}{j, runTest(tests[j], opts)}
				}
			}()
		}
		
		// Send work
		go func() {
			for i := range tests {
				ch <- struct {
					index int
					result TestResult
				}{i, TestResult{}}
			}
		}()
		
		// Collect results
		for i := 0; i < len(tests); i++ {
			item := <-ch
			results[item.index] = item.result
		}
	}

	return results
}

func runTest(test TestCase, opts TestOptions) TestResult {
	start := time.Now()
	result := TestResult{
		Name: test.Name,
	}

	timeout := opts.DefaultTimeout
	if test.Timeout != "" {
		if d, err := time.ParseDuration(test.Timeout); err == nil {
			timeout = d
		}
	}

	if opts.Verbose {
		fmt.Printf("Running: %s\n", test.Name)
	}

	// Build command
	cmd := exec.Command(test.Command[0], test.Command[1:]...)
	
	// Set working directory if specified
	if test.ConfigDir != "" {
		cmd.Dir = test.ConfigDir
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range test.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Run command with timeout
	done := make(chan error, 1)
	var output []byte
	var err error

	go func() {
		output, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case err = <-done:
		// Command completed
		result.Output = string(output)
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else if err != nil {
			result.Error = err.Error()
			result.ExitCode = -1
		}
	case <-time.After(timeout):
		// Command timed out
		cmd.Process.Kill()
		result.Error = fmt.Sprintf("Test timed out after %v", timeout)
		result.ExitCode = -1
	}

	result.Duration = time.Since(start)

	// Check expected exit code
	if result.ExitCode != test.ExpectedExit {
		result.Passed = false
		if result.Error == "" {
			result.Error = fmt.Sprintf("Expected exit code %d, got %d", test.ExpectedExit, result.ExitCode)
		}
	} else {
		result.Passed = validateOutput(result.Output, test.ExpectedOutput)
		if !result.Passed && result.Error == "" {
			result.Error = "Output validation failed"
		}
	}

	if opts.Verbose {
		if result.Passed {
			fmt.Printf("PASS: %s (%.2fs)\n", test.Name, result.Duration.Seconds())
		} else {
			fmt.Printf("FAIL: %s (%.2fs) - %s\n", test.Name, result.Duration.Seconds(), result.Error)
		}
	}

	return result
}

func validateOutput(output string, expected ExpectedOutput) bool {
	// Check ContainsAll
	for _, text := range expected.ContainsAll {
		if !strings.Contains(output, text) {
			return false
		}
	}

	// Check ContainsAny
	if len(expected.ContainsAny) > 0 {
		found := false
		for _, text := range expected.ContainsAny {
			if strings.Contains(output, text) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check NotContains
	for _, text := range expected.NotContains {
		if strings.Contains(output, text) {
			return false
		}
	}

	// Check Regex patterns
	for _, pattern := range expected.Regex {
		matched, err := regexp.MatchString(pattern, output)
		if err != nil {
			// Invalid regex pattern - fail the test
			return false
		}
		if !matched {
			return false
		}
	}

	// Check JSON path assertions
	if len(expected.JSONPath) > 0 {
		// Try to parse output as JSON
		var jsonData interface{}
		if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
			// Output is not valid JSON - fail if JSON assertions were expected
			return false
		}

		for _, assertion := range expected.JSONPath {
			if !validateJSONPath(jsonData, assertion) {
				return false
			}
		}
	}

	return true
}

// validateJSONPath validates a single JSON path assertion against parsed JSON data
func validateJSONPath(data interface{}, assertion JSONAssert) bool {
	value, found := navigateJSONPath(data, assertion.Path)
	if !found {
		return false
	}

	// Default assertion type is "equals"
	assertionType := assertion.Type
	if assertionType == "" {
		assertionType = "equals"
	}

	return compareJSONValues(value, assertion.Expected, assertionType)
}

// navigateJSONPath navigates through JSON structure using a path string
// Supports paths like: "field", "field.subfield", "array[0]", "field[0].subfield"
func navigateJSONPath(data interface{}, path string) (interface{}, bool) {
	if path == "" {
		return data, true
	}

	current := data
	remaining := path

	for remaining != "" {
		// Check for array index notation: field[index]
		if idx := strings.Index(remaining, "["); idx != -1 {
			// Get the field name before the bracket
			fieldName := remaining[:idx]
			if fieldName != "" {
				// Navigate to the field
				if m, ok := current.(map[string]interface{}); ok {
					var found bool
					current, found = m[fieldName]
					if !found {
						return nil, false
					}
				} else {
					return nil, false
				}
			}

			// Parse the array index
			endIdx := strings.Index(remaining, "]")
			if endIdx == -1 {
				return nil, false // malformed path
			}
			indexStr := remaining[idx+1 : endIdx]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, false
			}

			// Navigate into the array
			if arr, ok := current.([]interface{}); ok {
				if index < 0 || index >= len(arr) {
					return nil, false
				}
				current = arr[index]
			} else {
				return nil, false
			}

			// Move past the closing bracket
			remaining = remaining[endIdx+1:]
			// Skip the dot separator if present
			if strings.HasPrefix(remaining, ".") {
				remaining = remaining[1:]
			}
		} else {
			// No array notation, just field navigation
			// Find the next dot or end of string
			dotIdx := strings.Index(remaining, ".")
			var fieldName string
			if dotIdx == -1 {
				fieldName = remaining
				remaining = ""
			} else {
				fieldName = remaining[:dotIdx]
				remaining = remaining[dotIdx+1:]
			}

			// Navigate to the field
			if m, ok := current.(map[string]interface{}); ok {
				var found bool
				current, found = m[fieldName]
				if !found {
					return nil, false
				}
			} else {
				return nil, false
			}
		}
	}

	return current, true
}

// compareJSONValues compares two values based on the assertion type
func compareJSONValues(actual, expected interface{}, assertionType string) bool {
	switch assertionType {
	case "equals":
		return compareEquals(actual, expected)
	case "greater":
		return compareGreater(actual, expected)
	case "less":
		return compareLess(actual, expected)
	case "contains":
		return compareContains(actual, expected)
	default:
		// Unknown assertion type - default to equals
		return compareEquals(actual, expected)
	}
}

func compareEquals(actual, expected interface{}) bool {
	// Handle numeric comparisons (JSON numbers can be float64)
	if actualNum, ok := actual.(float64); ok {
		if expectedNum, ok := expected.(float64); ok {
			return actualNum == expectedNum
		}
		// Try converting expected to float
		if expectedStr, ok := expected.(string); ok {
			if expectedNum, err := strconv.ParseFloat(expectedStr, 64); err == nil {
				return actualNum == expectedNum
			}
		}
	}

	// Handle string comparisons
	if actualStr, ok := actual.(string); ok {
		if expectedStr, ok := expected.(string); ok {
			return actualStr == expectedStr
		}
	}

	// Handle boolean comparisons
	if actualBool, ok := actual.(bool); ok {
		if expectedBool, ok := expected.(bool); ok {
			return actualBool == expectedBool
		}
	}

	// Fallback to string representation comparison
	return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
}

func compareGreater(actual, expected interface{}) bool {
	actualNum, actualOk := toFloat64(actual)
	expectedNum, expectedOk := toFloat64(expected)
	if actualOk && expectedOk {
		return actualNum > expectedNum
	}
	return false
}

func compareLess(actual, expected interface{}) bool {
	actualNum, actualOk := toFloat64(actual)
	expectedNum, expectedOk := toFloat64(expected)
	if actualOk && expectedOk {
		return actualNum < expectedNum
	}
	return false
}

func compareContains(actual, expected interface{}) bool {
	// Check if actual array contains expected value
	if arr, ok := actual.([]interface{}); ok {
		for _, item := range arr {
			if compareEquals(item, expected) {
				return true
			}
		}
		return false
	}

	// Check if actual string contains expected substring
	if actualStr, ok := actual.(string); ok {
		if expectedStr, ok := expected.(string); ok {
			return strings.Contains(actualStr, expectedStr)
		}
	}

	return false
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func runCommand(cmdStr string, workDir string, timeout time.Duration) error {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("command timed out")
	}
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Minute // default
	}
	return d
}

func generateJSONReport(result SuiteResult, filename string) {
	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(filename, data, 0644)
}

func generateJUnitReport(result SuiteResult, filename string) {
	// Simplified JUnit XML generation
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="%s" tests="%d" failures="%d" time="%.2f">
`, result.Suite, result.Total, result.Failed, result.Duration.Seconds())

	for _, test := range result.Results {
		xml += fmt.Sprintf(`  <testcase name="%s" time="%.2f"`, test.Name, test.Duration.Seconds())
		if test.Passed {
			xml += " />\n"
		} else {
			xml += ">\n"
			xml += fmt.Sprintf("    <failure>%s</failure>\n", test.Error)
			xml += "  </testcase>\n"
		}
	}

	xml += "</testsuite>\n"
	os.WriteFile(filename, []byte(xml), 0644)
}

func printTextReport(result SuiteResult, verbose bool) {
	fmt.Printf("\nTest Results Summary\n")
	fmt.Printf("=======================\n")
	fmt.Printf("Suite: %s\n", result.Suite)
	fmt.Printf("Total: %d\n", result.Total)
	fmt.Printf("Passed: %d\n", result.Total-result.Failed)
	fmt.Printf("Failed: %d\n", result.Failed)
	fmt.Printf("Duration: %.2fs\n", result.Duration.Seconds())

	if result.Passed {
		fmt.Printf("Status: ALL PASSED\n")
	} else {
		fmt.Printf("Status: %d FAILED\n", result.Failed)
	}

	if verbose || result.Failed > 0 {
		fmt.Printf("\nDetailed Results:\n")
		for _, test := range result.Results {
			status := "PASS"
			if !test.Passed {
				status = "FAIL"
			}
			fmt.Printf("%s %s (%.2fs)", status, test.Name, test.Duration.Seconds())
			if !test.Passed {
				fmt.Printf(" - %s", test.Error)
			}
			fmt.Printf("\n")
		}
	}

	if result.Failed > 0 && verbose {
		fmt.Printf("\nFailed Test Details:\n")
		for _, test := range result.Results {
			if !test.Passed {
				fmt.Printf("\n--- %s ---\n", test.Name)
				fmt.Printf("Error: %s\n", test.Error)
				fmt.Printf("Exit Code: %d\n", test.ExitCode)
				fmt.Printf("Output:\n%s\n", test.Output)
			}
		}
	}
}