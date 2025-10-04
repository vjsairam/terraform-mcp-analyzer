package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Rule validation tool for TFUG rules packs
// Validates syntax, semantics, and consistency of rules

type Rule struct {
	ID        string                 `json:"id"`
	Ecosystem string                 `json:"ecosystem"`
	Provider  string                 `json:"provider,omitempty"`
	Module    string                 `json:"module,omitempty"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Fix       *FixAction             `json:"fix,omitempty"`
	State     *StateAction           `json:"state,omitempty"`
	Docs      []DocRef               `json:"docs,omitempty"`
	Meta      RuleMeta               `json:"meta"`
}

type FixAction struct {
	Codemod string                 `json:"codemod"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

type StateAction struct {
	Actions []StateOp `json:"actions"`
}

type StateOp struct {
	Op   string `json:"op"`
	Addr string `json:"addr"`
	Dest string `json:"dest,omitempty"`
	ID   string `json:"id,omitempty"`
}

type DocRef struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Excerpt string `json:"excerpt"`
}

type RuleMeta struct {
	Severity   string `json:"severity"`
	Confidence string `json:"confidence"`
}

type PackMeta struct {
	ID            string   `json:"id"`
	Version       string   `json:"version"`
	CreatedAt     string   `json:"created_at"`
	Channel       string   `json:"channel"`
	Digest        string   `json:"digest,omitempty"`
	Sources       []string `json:"sources,omitempty"`
	Builder       string   `json:"builder,omitempty"`
	SchemaVersion int      `json:"schema_version,omitempty"`
}

type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors"`
	Warnings []ValidationError `json:"warnings"`
	Stats    ValidationStats   `json:"stats"`
}

type ValidationError struct {
	Line    int    `json:"line"`
	RuleID  string `json:"rule_id,omitempty"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type ValidationStats struct {
	TotalRules     int `json:"total_rules"`
	ValidRules     int `json:"valid_rules"`
	HasMetadata    bool `json:"has_metadata"`
	RuleTypes      map[string]int `json:"rule_types"`
	Providers      map[string]int `json:"providers"`
	Modules        map[string]int `json:"modules"`
	SeverityLevels map[string]int `json:"severity_levels"`
}

var validRuleTypes = map[string]bool{
	"provider_min_version":       true,
	"module_merged":              true,
	"module_split":               true,
	"var_renamed":                true,
	"var_removed":                true,
	"var_added":                  true,
	"output_renamed":             true,
	"output_removed":             true,
	"resource_renamed":           true,
	"argument_renamed":           true,
	"argument_removed":           true,
	"argument_added":             true,
	"argument_structure_changed": true,
	"state_move":                 true,
	"behavior_change":            true,
	"dependency_change":          true,
}

var validSeverities = map[string]bool{
	"breaking":  true,
	"advisory":  true,
	"info":      true,
}

var validConfidences = map[string]bool{
	"high": true,
	"med":  true,
	"low":  true,
}

var validStateOps = map[string]bool{
	"rm":     true,
	"mv":     true,
	"import": true,
}

func main() {
	var (
		packFile   = flag.String("pack", "", "Rules pack file to validate")
		strict     = flag.Bool("strict", false, "Enable strict validation mode")
		format     = flag.String("format", "text", "Output format: text, json")
		verbose    = flag.Bool("verbose", false, "Verbose output")
		checkDups  = flag.Bool("check-duplicates", true, "Check for duplicate rule IDs")
		checkRefs  = flag.Bool("check-references", true, "Check documentation URLs")
	)
	flag.Parse()

	if *packFile == "" {
		log.Fatal("Please specify a rules pack file with -pack")
	}

	result := validateRulesPack(*packFile, ValidationOptions{
		Strict:          *strict,
		CheckDuplicates: *checkDups,
		CheckReferences: *checkRefs,
		Verbose:         *verbose,
	})

	switch *format {
	case "json":
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))
	case "text":
		printTextReport(result, *verbose)
	default:
		log.Fatalf("Unknown output format: %s", *format)
	}

	if !result.Valid {
		os.Exit(1)
	}
}

type ValidationOptions struct {
	Strict          bool
	CheckDuplicates bool
	CheckReferences bool
	Verbose         bool
}

func validateRulesPack(filename string, opts ValidationOptions) ValidationResult {
	file, err := os.Open(filename)
	if err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{Line: 0, Type: "file_error", Message: fmt.Sprintf("Cannot open file: %v", err)},
			},
		}
	}
	defer file.Close()

	var result ValidationResult
	var stats ValidationStats
	var seenIDs = make(map[string]int)
	
	stats.RuleTypes = make(map[string]int)
	stats.Providers = make(map[string]int)
	stats.Modules = make(map[string]int)
	stats.SeverityLevels = make(map[string]int)

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse JSON
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Type:    "syntax_error",
				Message: fmt.Sprintf("Invalid JSON: %v", err),
			})
			continue
		}

		// Check if this is metadata
		if _, hasMeta := data["_meta"]; hasMeta {
			stats.HasMetadata = true
			if err := validateMetadata(data, lineNum, &result, opts); err != nil {
				result.Errors = append(result.Errors, *err)
			}
			continue
		}

		// Parse as rule
		var rule Rule
		if err := json.Unmarshal([]byte(line), &rule); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Type:    "parse_error",
				Message: fmt.Sprintf("Cannot parse rule: %v", err),
			})
			continue
		}

		// Validate rule
		errors, warnings := validateRule(rule, lineNum, opts)
		result.Errors = append(result.Errors, errors...)
		result.Warnings = append(result.Warnings, warnings...)

		// Track statistics
		stats.TotalRules++
		if len(errors) == 0 {
			stats.ValidRules++
		}

		stats.RuleTypes[rule.Type]++
		if rule.Provider != "" {
			stats.Providers[rule.Provider]++
		}
		if rule.Module != "" {
			stats.Modules[rule.Module]++
		}
		stats.SeverityLevels[rule.Meta.Severity]++

		// Check for duplicate IDs
		if opts.CheckDuplicates {
			if prevLine, exists := seenIDs[rule.ID]; exists {
				result.Errors = append(result.Errors, ValidationError{
					Line:    lineNum,
					RuleID:  rule.ID,
					Type:    "duplicate_id",
					Message: fmt.Sprintf("Duplicate rule ID (first seen on line %d)", prevLine),
				})
			} else {
				seenIDs[rule.ID] = lineNum
			}
		}
	}

	if err := scanner.Err(); err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Line:    0,
			Type:    "read_error",
			Message: fmt.Sprintf("Error reading file: %v", err),
		})
	}

	result.Valid = len(result.Errors) == 0
	result.Stats = stats

	return result
}

func validateRule(rule Rule, lineNum int, opts ValidationOptions) ([]ValidationError, []ValidationError) {
	var errors []ValidationError
	var warnings []ValidationError

	// Required fields
	if rule.ID == "" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_field", Message: "Rule ID is required",
		})
	} else {
		// Validate ID format
		if !strings.HasPrefix(rule.ID, "tfug.") {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "invalid_format", Message: "Rule ID must start with 'tfug.'",
			})
		}
	}

	if rule.Ecosystem == "" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_field", Message: "Ecosystem is required",
		})
	} else if rule.Ecosystem != "terraform" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "invalid_value", Message: "Ecosystem must be 'terraform'",
		})
	}

	if rule.From == "" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_field", Message: "From version constraint is required",
		})
	} else {
		if _, err := semver.NewConstraint(rule.From); err != nil {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "invalid_constraint", Message: fmt.Sprintf("Invalid 'from' constraint: %v", err),
			})
		}
	}

	if rule.To == "" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_field", Message: "To version constraint is required",
		})
	} else {
		if _, err := semver.NewConstraint(rule.To); err != nil {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "invalid_constraint", Message: fmt.Sprintf("Invalid 'to' constraint: %v", err),
			})
		}
	}

	if rule.Type == "" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_field", Message: "Rule type is required",
		})
	} else if !validRuleTypes[rule.Type] {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "invalid_type", 
			Message: fmt.Sprintf("Unknown rule type: %s", rule.Type),
		})
	}

	// Validate meta fields
	if rule.Meta.Severity == "" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_field", Message: "Meta.severity is required",
		})
	} else if !validSeverities[rule.Meta.Severity] {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "invalid_value", 
			Message: fmt.Sprintf("Invalid severity: %s", rule.Meta.Severity),
		})
	}

	if rule.Meta.Confidence == "" {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_field", Message: "Meta.confidence is required",
		})
	} else if !validConfidences[rule.Meta.Confidence] {
		errors = append(errors, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "invalid_value", 
			Message: fmt.Sprintf("Invalid confidence: %s", rule.Meta.Confidence),
		})
	}

	// Provider or module should be specified (unless it's a generic rule)
	if rule.Provider == "" && rule.Module == "" && !strings.Contains(rule.ID, ".core.") {
		warnings = append(warnings, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_scope", 
			Message: "Neither provider nor module specified - rule may be too generic",
		})
	}

	// Type-specific validations
	switch rule.Type {
	case "provider_min_version":
		if rule.Provider == "" {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_field", 
				Message: "Provider is required for provider_min_version rules",
			})
		}
		if rule.Payload["min"] == nil {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_payload", 
				Message: "payload.min is required for provider_min_version rules",
			})
		}

	case "var_renamed":
		if rule.Payload["from"] == nil || rule.Payload["to"] == nil {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_payload", 
				Message: "payload.from and payload.to are required for var_renamed rules",
			})
		}

	case "var_removed":
		if rule.Payload["name"] == nil {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_payload", 
				Message: "payload.name is required for var_removed rules",
			})
		}

	case "state_move":
		if rule.State == nil || len(rule.State.Actions) == 0 {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_state", 
				Message: "state.actions are required for state_move rules",
			})
		} else {
			for i, op := range rule.State.Actions {
				if !validStateOps[op.Op] {
					errors = append(errors, ValidationError{
						Line: lineNum, RuleID: rule.ID, Type: "invalid_state_op", 
						Message: fmt.Sprintf("Invalid state operation in action %d: %s", i, op.Op),
					})
				}
				if op.Addr == "" {
					errors = append(errors, ValidationError{
						Line: lineNum, RuleID: rule.ID, Type: "missing_state_addr", 
						Message: fmt.Sprintf("state.actions[%d].addr is required", i),
					})
				}
				if op.Op == "mv" && op.Dest == "" {
					errors = append(errors, ValidationError{
						Line: lineNum, RuleID: rule.ID, Type: "missing_state_dest", 
						Message: fmt.Sprintf("state.actions[%d].dest is required for mv operations", i),
					})
				}
			}
		}
	}

	// Validate documentation
	if len(rule.Docs) == 0 && opts.Strict {
		warnings = append(warnings, ValidationError{
			Line: lineNum, RuleID: rule.ID, Type: "missing_docs", 
			Message: "No documentation references provided",
		})
	}

	for i, doc := range rule.Docs {
		if doc.URL == "" {
			warnings = append(warnings, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_doc_url", 
				Message: fmt.Sprintf("Documentation reference %d missing URL", i),
			})
		}
		if doc.Title == "" {
			warnings = append(warnings, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_doc_title", 
				Message: fmt.Sprintf("Documentation reference %d missing title", i),
			})
		}
	}

	// Validate fix actions
	if rule.Fix != nil {
		if rule.Fix.Codemod == "" {
			errors = append(errors, ValidationError{
				Line: lineNum, RuleID: rule.ID, Type: "missing_fix_codemod", 
				Message: "fix.codemod is required when fix is specified",
			})
		}
	}

	return errors, warnings
}

func validateMetadata(data map[string]interface{}, lineNum int, result *ValidationResult, opts ValidationOptions) *ValidationError {
	meta, ok := data["_meta"].(map[string]interface{})
	if !ok {
		return &ValidationError{
			Line: lineNum, Type: "invalid_metadata", Message: "_meta must be an object",
		}
	}

	// Check required metadata fields
	requiredFields := []string{"id", "version", "created_at", "channel"}
	for _, field := range requiredFields {
		if _, exists := meta[field]; !exists {
			return &ValidationError{
				Line: lineNum, Type: "missing_metadata", Message: fmt.Sprintf("_meta.%s is required", field),
			}
		}
	}

	// Validate channel
	if channel, ok := meta["channel"].(string); ok {
		validChannels := map[string]bool{"stable": true, "rc": true, "dev": true}
		if !validChannels[channel] {
			return &ValidationError{
				Line: lineNum, Type: "invalid_channel", Message: fmt.Sprintf("Invalid channel: %s", channel),
			}
		}
	}

	return nil
}

func printTextReport(result ValidationResult, verbose bool) {
	fmt.Printf("TFUG Rules Pack Validation Report\n")
	fmt.Printf("====================================\n\n")

	// Summary
	if result.Valid {
		fmt.Printf("Status: VALID\n")
	} else {
		fmt.Printf("Status: INVALID (%d errors)\n", len(result.Errors))
	}

	fmt.Printf("Rules: %d total, %d valid\n", result.Stats.TotalRules, result.Stats.ValidRules)
	if result.Stats.HasMetadata {
		fmt.Printf("Metadata: Present\n")
	} else {
		fmt.Printf("Metadata: Missing\n")
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings: %d\n", len(result.Warnings))
	}

	fmt.Printf("\n")

	// Statistics
	if verbose {
		fmt.Printf("Statistics:\n")
		
		if len(result.Stats.RuleTypes) > 0 {
			fmt.Printf("  Rule Types:\n")
			for ruleType, count := range result.Stats.RuleTypes {
				fmt.Printf("    %s: %d\n", ruleType, count)
			}
		}
		
		if len(result.Stats.Providers) > 0 {
			fmt.Printf("  Providers:\n")
			for provider, count := range result.Stats.Providers {
				fmt.Printf("    %s: %d\n", provider, count)
			}
		}
		
		if len(result.Stats.Modules) > 0 {
			fmt.Printf("  Modules:\n")
			for module, count := range result.Stats.Modules {
				fmt.Printf("    %s: %d\n", module, count)
			}
		}
		
		if len(result.Stats.SeverityLevels) > 0 {
			fmt.Printf("  Severity Levels:\n")
			for severity, count := range result.Stats.SeverityLevels {
				fmt.Printf("    %s: %d\n", severity, count)
			}
		}
		
		fmt.Printf("\n")
	}

	// Errors
	if len(result.Errors) > 0 {
		fmt.Printf("Errors:\n")
		for _, err := range result.Errors {
			fmt.Printf("  Line %d", err.Line)
			if err.RuleID != "" {
				fmt.Printf(" [%s]", err.RuleID)
			}
			fmt.Printf(": %s - %s\n", err.Type, err.Message)
		}
		fmt.Printf("\n")
	}

	// Warnings  
	if len(result.Warnings) > 0 && verbose {
		fmt.Printf("Warnings:\n")
		for _, warn := range result.Warnings {
			fmt.Printf("  Line %d", warn.Line)
			if warn.RuleID != "" {
				fmt.Printf(" [%s]", warn.RuleID)
			}
			fmt.Printf(": %s - %s\n", warn.Type, warn.Message)
		}
		fmt.Printf("\n")
	}

	// Recommendations
	if result.Valid && len(result.Warnings) == 0 {
		fmt.Printf("Excellent! Your rules pack passes all validations.\n")
	} else if result.Valid {
		fmt.Printf("Good! Your rules pack is valid but has some warnings to consider.\n")
	} else {
		fmt.Printf("Please fix the errors above before using this rules pack.\n")
	}
}