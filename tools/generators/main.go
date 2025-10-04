package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Configuration structures for template generation
type GeneratorConfig struct {
	Type           string `json:"type"`           // "provider" or "module"
	Provider       string `json:"provider"`       // e.g., "azurerm", "aws"
	ProviderSource string `json:"provider_source"` // e.g., "hashicorp/azurerm"
	FromVersion    string `json:"from_version"`   // e.g., "v2.99"
	ToVersion      string `json:"to_version"`     // e.g., "v3.0"
	Module         string `json:"module,omitempty"` // For module templates
	OutputDir      string `json:"output_dir"`
	
	// Template-specific data
	TemplateData interface{} `json:"template_data"`
}

type ProviderTemplateData struct {
	Provider              string                 `json:"provider"`
	ProviderTitle         string                 `json:"provider_title"`
	ProviderSource        string                 `json:"provider_source"`
	FromVersion           string                 `json:"from_version"`
	ToVersion             string                 `json:"to_version"`
	FromVersionConstraint string                 `json:"from_version_constraint"`
	ToVersionConstraint   string                 `json:"to_version_constraint"`
	ProviderConfig        []KeyValue             `json:"provider_config"`
	Resources             []Resource             `json:"resources"`
	DataSources           []DataSource           `json:"data_sources"`
	Locals                []KeyValue             `json:"locals,omitempty"`
	Variables             bool                   `json:"variables,omitempty"`
}

type ModuleTemplateData struct {
	Provider        string   `json:"provider"`
	ProviderSource  string   `json:"provider_source"`
	ProviderVersion string   `json:"provider_version"`
	FromVersion     string   `json:"from_version"`
	ToVersion       string   `json:"to_version"`
	Modules         []Module `json:"modules"`
	Outputs         []Output `json:"outputs,omitempty"`
	Locals          []KeyValue `json:"locals,omitempty"`
}

type RulesTemplateData struct {
	PackID            string              `json:"pack_id"`
	Version           string              `json:"version"`
	CreatedAt         string              `json:"created_at"`
	Channel           string              `json:"channel"`
	Sources           []string            `json:"sources"`
	Builder           string              `json:"builder"`
	RulePrefix        string              `json:"rule_prefix"`
	ProviderName      string              `json:"provider_name,omitempty"`
	ModuleSource      string              `json:"module_source,omitempty"`
	FromVersion       string              `json:"from_version"`
	ToVersion         string              `json:"to_version"`
	FromConstraint    string              `json:"from_constraint"`
	ToConstraint      string              `json:"to_constraint"`
	MinVersion        string              `json:"min_version,omitempty"`
	UpgradeGuideURL   string              `json:"upgrade_guide_url,omitempty"`
	
	// Rule types
	ResourceRenames   []ResourceRename   `json:"resource_renames,omitempty"`
	ArgumentChanges   []ArgumentChange   `json:"argument_changes,omitempty"`
	BehaviorChanges   []BehaviorChange   `json:"behavior_changes,omitempty"`
	StateOperations   []StateOperation   `json:"state_operations,omitempty"`
	VariableRenames   []VariableRename   `json:"variable_renames,omitempty"`
	VariableRemovals  []VariableRemoval  `json:"variable_removals,omitempty"`
	OutputChanges     []OutputChange     `json:"output_changes,omitempty"`
	ModuleChanges     []ModuleChange     `json:"module_changes,omitempty"`
	ResourceChanges   []ResourceChange   `json:"resource_changes,omitempty"`
}

type TestTemplateData struct {
	Provider                  string              `json:"provider"`
	ProviderTitle             string              `json:"provider_title"`
	FromVersion               string              `json:"from_version"`
	ToVersion                 string              `json:"to_version"`
	RulePrefix                string              `json:"rule_prefix"`
	ExpectedBasicFindings     int                 `json:"expected_basic_findings"`
	ExpectedRuleIDs           []string            `json:"expected_rule_ids"`
	HasComplexScenario        bool                `json:"has_complex_scenario,omitempty"`
	ExpectedComplexFindings   int                 `json:"expected_complex_findings,omitempty"`
	ExpectedComplexRuleIDs    []string            `json:"expected_complex_rule_ids,omitempty"`
	ResourceTests             []ResourceTest      `json:"resource_tests,omitempty"`
	HasCodemods               bool                `json:"has_codemods,omitempty"`
	CodemodTests              []CodemodTest       `json:"codemod_tests,omitempty"`
	HasStateOps               bool                `json:"has_state_ops,omitempty"`
	StateOpTests              []StateOpTest       `json:"state_op_tests,omitempty"`
	CustomTests               []CustomTest        `json:"custom_tests,omitempty"`
}

// Supporting structures
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Resource struct {
	Type           string     `json:"type"`
	Name           string     `json:"name"`
	Comment        string     `json:"comment,omitempty"`
	Arguments      []KeyValue `json:"arguments"`
	DeprecatedArgs []DeprecatedArg `json:"deprecated_args,omitempty"`
}

type DeprecatedArg struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	ChangeType string `json:"change_type"` // "renamed", "removed", "changed"
	Note       string `json:"note"`
}

type DataSource struct {
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Arguments []KeyValue `json:"arguments"`
}

type Module struct {
	Name                string          `json:"name"`
	Source              string          `json:"source"`
	FromVersion         string          `json:"from_version"`
	Variables           []KeyValue      `json:"variables"`
	DeprecatedVariables []DeprecatedArg `json:"deprecated_variables,omitempty"`
	RemovedVariables    []RemovedVar    `json:"removed_variables,omitempty"`
}

type RemovedVar struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Replacement string `json:"replacement"`
}

type Output struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Value       string `json:"value"`
}

// Rule structures
type ResourceRename struct {
	Sequence     string `json:"sequence"`
	ResourceType string `json:"resource_type"`
	OldName      string `json:"old_name"`
	NewName      string `json:"new_name"`
	Condition    string `json:"condition,omitempty"`
	Title        string `json:"title"`
	DocsURL      string `json:"docs_url"`
	Excerpt      string `json:"excerpt"`
	Confidence   string `json:"confidence"`
}

type ArgumentChange struct {
	Sequence     string           `json:"sequence"`
	ResourceType string           `json:"resource_type"`
	ChangeType   string           `json:"change_type"` // "renamed", "removed", "structure_changed"
	ArgumentName string           `json:"argument_name"`
	NewName      string           `json:"new_name,omitempty"`
	Note         string           `json:"note,omitempty"`
	Fix          *FixAction       `json:"fix,omitempty"`
	Title        string           `json:"title"`
	DocsURL      string           `json:"docs_url"`
	Excerpt      string           `json:"excerpt"`
	Severity     string           `json:"severity"`
	Confidence   string           `json:"confidence"`
}

type FixAction struct {
	Type string          `json:"type"`
	Args json.RawMessage `json:"args"`
}

type BehaviorChange struct {
	Sequence   string `json:"sequence"`
	Component  string `json:"component"`
	Resource   string `json:"resource,omitempty"`
	Change     string `json:"change"`
	Impact     string `json:"impact,omitempty"`
	Title      string `json:"title"`
	DocsURL    string `json:"docs_url"`
	Excerpt    string `json:"excerpt"`
	Severity   string `json:"severity"`
	Confidence string `json:"confidence"`
}

type StateOperation struct {
	Sequence   string      `json:"sequence"`
	Component  string      `json:"component"`
	Actions    []StateAction `json:"actions"`
	Title      string      `json:"title"`
	DocsURL    string      `json:"docs_url"`
	Excerpt    string      `json:"excerpt"`
	Confidence string      `json:"confidence"`
}

type StateAction struct {
	Op   string `json:"op"`   // "rm", "mv", "import"
	Addr string `json:"addr"`
	Dest string `json:"dest,omitempty"`
	ID   string `json:"id,omitempty"`
}

type VariableRename struct {
	Sequence    string `json:"sequence"`
	ModuleName  string `json:"module_name"`
	ModuleSource string `json:"module_source"`
	OldName     string `json:"old_name"`
	NewName     string `json:"new_name"`
	TypeChange  string `json:"type_change,omitempty"`
	Title       string `json:"title"`
	DocsURL     string `json:"docs_url"`
	Excerpt     string `json:"excerpt"`
	Confidence  string `json:"confidence"`
}

type VariableRemoval struct {
	Sequence     string `json:"sequence"`
	ModuleName   string `json:"module_name"`
	ModuleSource string `json:"module_source"`
	Name         string `json:"name"`
	Replacement  string `json:"replacement,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Title        string `json:"title"`
	DocsURL      string `json:"docs_url"`
	Excerpt      string `json:"excerpt"`
	Confidence   string `json:"confidence"`
}

type OutputChange struct {
	Sequence     string `json:"sequence"`
	ModuleName   string `json:"module_name"`
	ModuleSource string `json:"module_source"`
	ChangeType   string `json:"change_type"` // "renamed", "removed", "type_changed"
	Name         string `json:"name"`
	NewName      string `json:"new_name,omitempty"`
	TypeChange   string `json:"type_change,omitempty"`
	Note         string `json:"note,omitempty"`
	Title        string `json:"title"`
	DocsURL      string `json:"docs_url"`
	Excerpt      string `json:"excerpt"`
	Severity     string `json:"severity"`
	Confidence   string `json:"confidence"`
}

type ModuleChange struct {
	Sequence     string   `json:"sequence"`
	ModuleName   string   `json:"module_name"`
	ModuleSource string   `json:"module_source"`
	ChangeType   string   `json:"change_type"` // "merged", "split", "moved"
	From         string   `json:"from,omitempty"`
	To           string   `json:"to,omitempty"`
	OldModule    string   `json:"old_module,omitempty"`
	NewModules   []string `json:"new_modules,omitempty"`
	Note         string   `json:"note,omitempty"`
	Fix          *FixAction `json:"fix,omitempty"`
	Title        string   `json:"title"`
	DocsURL      string   `json:"docs_url"`
	Excerpt      string   `json:"excerpt"`
	Confidence   string   `json:"confidence"`
}

type ResourceChange struct {
	Sequence     string        `json:"sequence"`
	ModuleName   string        `json:"module_name"`
	ModuleSource string        `json:"module_source"`
	ChangeType   string        `json:"change_type"` // "moved", "renamed", "removed"
	ResourceType string        `json:"resource_type"`
	OldAddress   string        `json:"old_address,omitempty"`
	NewAddress   string        `json:"new_address,omitempty"`
	Note         string        `json:"note,omitempty"`
	StateOps     []StateAction `json:"state_ops,omitempty"`
	Title        string        `json:"title"`
	DocsURL      string        `json:"docs_url"`
	Excerpt      string        `json:"excerpt"`
	Confidence   string        `json:"confidence"`
}

// Test structures
type ResourceTest struct {
	ResourceType     string           `json:"resource_type"`
	TestType         string           `json:"test_type"`
	RulesFile        string           `json:"rules_file"`
	ConfigDir        string           `json:"config_dir"`
	ExpectedFindings []ExpectedFinding `json:"expected_findings"`
}

type ExpectedFinding struct {
	RuleID          string `json:"rule_id"`
	Severity        string `json:"severity"`
	MessageContains string `json:"message_contains"`
}

type CodemodTest struct {
	Name         string `json:"name"`
	ConfigDir    string `json:"config_dir"`
	RulesFile    string `json:"rules_file"`
	ExpectedDiff string `json:"expected_diff"`
}

type StateOpTest struct {
	Name        string      `json:"name"`
	ConfigDir   string      `json:"config_dir"`
	RulesFile   string      `json:"rules_file"`
	ExpectedOps []StateAction `json:"expected_ops"`
}

type CustomTest struct {
	Name           string `json:"name"`
	Implementation string `json:"implementation"`
}

func main() {
	var (
		configFile   = flag.String("config", "", "Configuration file path")
		templateType = flag.String("type", "", "Template type: provider, module, rules, test")
		provider     = flag.String("provider", "", "Provider name (e.g., azurerm, aws)")
		fromVersion  = flag.String("from", "", "From version (e.g., v2.99)")
		toVersion    = flag.String("to", "", "To version (e.g., v3.0)")
		outputDir    = flag.String("output", "", "Output directory")
		interactive  = flag.Bool("interactive", false, "Interactive mode")
		listTemplates = flag.Bool("list", false, "List available templates")
	)
	flag.Parse()

	if *listTemplates {
		listAvailableTemplates()
		return
	}

	if *configFile != "" {
		generateFromConfig(*configFile)
		return
	}

	if *interactive {
		runInteractiveMode()
		return
	}

	if *templateType == "" || *provider == "" || *fromVersion == "" || *toVersion == "" || *outputDir == "" {
		fmt.Println("Usage: generator -type <type> -provider <provider> -from <version> -to <version> -output <dir>")
		fmt.Println("   or: generator -config <config.json>")
		fmt.Println("   or: generator -interactive")
		fmt.Println("   or: generator -list")
		flag.PrintDefaults()
		os.Exit(1)
	}

	config := GeneratorConfig{
		Type:        *templateType,
		Provider:    *provider,
		FromVersion: *fromVersion,
		ToVersion:   *toVersion,
		OutputDir:   *outputDir,
	}

	if err := generateTemplates(config); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Printf("Successfully generated %s templates for %s (%s -> %s) in %s\n",
		*templateType, *provider, *fromVersion, *toVersion, *outputDir)
}

func generateFromConfig(configPath string) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config GeneratorConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	if err := generateTemplates(config); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Printf("Successfully generated templates from config file\n")
}

func runInteractiveMode() {
	scanner := bufio.NewScanner(os.Stdin)
	
	fmt.Println("TFUG Template Generator - Interactive Mode")
	fmt.Println()

	// Get template type
	fmt.Print("Template type (provider/module/rules/test): ")
	scanner.Scan()
	templateType := strings.TrimSpace(scanner.Text())

	// Get provider
	fmt.Print("Provider name (e.g., azurerm, aws, gcp): ")
	scanner.Scan()
	provider := strings.TrimSpace(scanner.Text())

	// Get versions
	fmt.Print("From version (e.g., v2.99): ")
	scanner.Scan()
	fromVersion := strings.TrimSpace(scanner.Text())

	fmt.Print("To version (e.g., v3.0): ")
	scanner.Scan()
	toVersion := strings.TrimSpace(scanner.Text())

	// Get output directory
	fmt.Print("Output directory: ")
	scanner.Scan()
	outputDir := strings.TrimSpace(scanner.Text())

	config := GeneratorConfig{
		Type:        templateType,
		Provider:    provider,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		OutputDir:   outputDir,
	}

	// Interactive template data collection based on type
	switch templateType {
	case "provider":
		config.TemplateData = collectProviderTemplateData(scanner, provider, fromVersion, toVersion)
	case "module":
		config.TemplateData = collectModuleTemplateData(scanner, provider, fromVersion, toVersion)
	case "rules":
		config.TemplateData = collectRulesTemplateData(scanner, provider, fromVersion, toVersion)
	case "test":
		config.TemplateData = collectTestTemplateData(scanner, provider, fromVersion, toVersion)
	default:
		log.Fatalf("Unknown template type: %s", templateType)
	}

	if err := generateTemplates(config); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Printf("\nSuccessfully generated %s templates for %s (%s -> %s)\n",
		templateType, provider, fromVersion, toVersion)
}

func listAvailableTemplates() {
	fmt.Println("📋 Available Templates:")
	fmt.Println()
	fmt.Println("1. provider - Generate provider test configurations")
	fmt.Println("   - Basic provider upgrade scenarios")
	fmt.Println("   - Resource and data source examples")
	fmt.Println("   - Lock files and variable configurations")
	fmt.Println()
	fmt.Println("2. module - Generate module upgrade test cases")
	fmt.Println("   - Module version transitions")
	fmt.Println("   - Variable and output changes")
	fmt.Println("   - Complex module scenarios")
	fmt.Println()
	fmt.Println("3. rules - Generate rules pack templates")
	fmt.Println("   - Provider version rules")
	fmt.Println("   - Resource and argument changes")
	fmt.Println("   - Behavior change documentation")
	fmt.Println()
	fmt.Println("4. test - Generate Go test files")
	fmt.Println("   - Unit test templates")
	fmt.Println("   - Integration test scenarios")
	fmt.Println("   - Performance benchmarks")
}

func generateTemplates(config GeneratorConfig) error {
	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	switch config.Type {
	case "provider":
		return generateProviderTemplates(config)
	case "module":
		return generateModuleTemplates(config)
	case "rules":
		return generateRulesTemplates(config)
	case "test":
		return generateTestTemplates(config)
	default:
		return fmt.Errorf("unknown template type: %s", config.Type)
	}
}

func generateProviderTemplates(config GeneratorConfig) error {
	// Use provided template data or create default
	var data ProviderTemplateData
	if config.TemplateData != nil {
		templateBytes, _ := json.Marshal(config.TemplateData)
		json.Unmarshal(templateBytes, &data)
	} else {
		data = createDefaultProviderTemplateData(config)
	}

	// Generate main.tf
	if err := generateFromTemplate(
		"templates/terraform/provider_basic.tf.template",
		filepath.Join(config.OutputDir, "main.tf"),
		data,
	); err != nil {
		return fmt.Errorf("failed to generate main.tf: %v", err)
	}

	// Generate lock file
	lockData := struct {
		Provider string
		Version  string
		Hashes   []string
	}{
		Provider: data.ProviderSource,
		Version:  strings.TrimPrefix(data.FromVersion, "v"),
		Hashes: []string{
			"h1:example1234567890abcdef=",
			"h1:example0987654321fedcba=",
		},
	}

	lockTemplate := `# This file is maintained automatically by "terraform init".

provider "registry.terraform.io/{{.Provider}}" {
  version     = "{{.Version}}"
  constraints = "~> {{.Version}}"
  hashes = [
{{range .Hashes}}    "{{.}}",
{{end}}  ]
}
`

	tmpl, err := template.New("lockfile").Parse(lockTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse lock template: %v", err)
	}

	lockFile, err := os.Create(filepath.Join(config.OutputDir, ".terraform.lock.hcl"))
	if err != nil {
		return fmt.Errorf("failed to create lock file: %v", err)
	}
	defer lockFile.Close()

	if err := tmpl.Execute(lockFile, lockData); err != nil {
		return fmt.Errorf("failed to execute lock template: %v", err)
	}

	fmt.Printf("Generated provider templates in %s\n", config.OutputDir)
	return nil
}

func generateModuleTemplates(config GeneratorConfig) error {
	var data ModuleTemplateData
	if config.TemplateData != nil {
		templateBytes, _ := json.Marshal(config.TemplateData)
		json.Unmarshal(templateBytes, &data)
	} else {
		data = createDefaultModuleTemplateData(config)
	}

	return generateFromTemplate(
		"templates/terraform/module_upgrade.tf.template",
		filepath.Join(config.OutputDir, "main.tf"),
		data,
	)
}

func generateRulesTemplates(config GeneratorConfig) error {
	var data RulesTemplateData
	if config.TemplateData != nil {
		templateBytes, _ := json.Marshal(config.TemplateData)
		json.Unmarshal(templateBytes, &data)
	} else {
		data = createDefaultRulesTemplateData(config)
	}

	templateFile := "templates/rules/provider_rules.jsonl.template"
	if config.Type == "module" {
		templateFile = "templates/rules/module_rules.jsonl.template"
	}

	outputFile := fmt.Sprintf("%s_%s_to_%s.jsonl", 
		config.Provider, 
		strings.TrimPrefix(config.FromVersion, "v"),
		strings.TrimPrefix(config.ToVersion, "v"))

	return generateFromTemplate(
		templateFile,
		filepath.Join(config.OutputDir, outputFile),
		data,
	)
}

func generateTestTemplates(config GeneratorConfig) error {
	var data TestTemplateData
	if config.TemplateData != nil {
		templateBytes, _ := json.Marshal(config.TemplateData)
		json.Unmarshal(templateBytes, &data)
	} else {
		data = createDefaultTestTemplateData(config)
	}

	outputFile := fmt.Sprintf("%s_%s_to_%s_test.go",
		config.Provider,
		strings.TrimPrefix(config.FromVersion, "v"),
		strings.TrimPrefix(config.ToVersion, "v"))

	return generateFromTemplate(
		"templates/tests/provider_test.go.template",
		filepath.Join(config.OutputDir, outputFile),
		data,
	)
}

func generateFromTemplate(templatePath, outputPath string, data interface{}) error {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %v", templatePath, err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %v", outputPath, err)
	}
	defer outputFile.Close()

	if err := tmpl.Execute(outputFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %v", err)
	}

	return nil
}

// Default template data creators
func createDefaultProviderTemplateData(config GeneratorConfig) ProviderTemplateData {
	providerSource := fmt.Sprintf("hashicorp/%s", config.Provider)
	if config.ProviderSource != "" {
		providerSource = config.ProviderSource
	}

	return ProviderTemplateData{
		Provider:              config.Provider,
		ProviderTitle:         strings.Title(config.Provider),
		ProviderSource:        providerSource,
		FromVersion:           config.FromVersion,
		ToVersion:             config.ToVersion,
		FromVersionConstraint: fmt.Sprintf("~> %s", strings.TrimPrefix(config.FromVersion, "v")),
		ToVersionConstraint:   fmt.Sprintf(">= %s", strings.TrimPrefix(config.ToVersion, "v")),
		ProviderConfig: []KeyValue{
			{Key: "features", Value: "{}"},
		},
		Resources: []Resource{
			{
				Type:    fmt.Sprintf("%s_resource_group", config.Provider),
				Name:    "example",
				Comment: "Example resource that may have breaking changes",
				Arguments: []KeyValue{
					{Key: "name", Value: "\"example-rg\""},
					{Key: "location", Value: "\"West Europe\""},
				},
			},
		},
		Variables: true,
	}
}

func createDefaultModuleTemplateData(config GeneratorConfig) ModuleTemplateData {
	return ModuleTemplateData{
		Provider:        config.Provider,
		ProviderSource:  fmt.Sprintf("hashicorp/%s", config.Provider),
		ProviderVersion: "~> 4.0",
		FromVersion:     config.FromVersion,
		ToVersion:       config.ToVersion,
		Modules: []Module{
			{
				Name:        "example",
				Source:      fmt.Sprintf("terraform-%s-modules/example/%s", config.Provider, config.Provider),
				FromVersion: strings.TrimPrefix(config.FromVersion, "v"),
				Variables: []KeyValue{
					{Key: "name", Value: "\"example\""},
					{Key: "location", Value: "var.location"},
				},
			},
		},
	}
}

func createDefaultRulesTemplateData(config GeneratorConfig) RulesTemplateData {
	now := time.Now()
	return RulesTemplateData{
		PackID:         fmt.Sprintf("tfug.%s.%s_to_%s", config.Provider, config.FromVersion, config.ToVersion),
		Version:        fmt.Sprintf("%d.%02d.1", now.Year(), now.Month()),
		CreatedAt:      now.Format(time.RFC3339),
		Channel:        "stable",
		Sources:        []string{fmt.Sprintf("https://registry.terraform.io/providers/hashicorp/%s", config.Provider)},
		Builder:        "tfug-generator",
		RulePrefix:     fmt.Sprintf("tfug.%s", config.Provider),
		ProviderName:   fmt.Sprintf("hashicorp/%s", config.Provider),
		FromVersion:    config.FromVersion,
		ToVersion:      config.ToVersion,
		FromConstraint: fmt.Sprintf(">=%s <=%s", strings.TrimPrefix(config.FromVersion, "v"), strings.TrimPrefix(config.FromVersion, "v")),
		ToConstraint:   fmt.Sprintf(">=%s", strings.TrimPrefix(config.ToVersion, "v")),
		MinVersion:     strings.TrimPrefix(config.ToVersion, "v"),
		UpgradeGuideURL: fmt.Sprintf("https://registry.terraform.io/providers/hashicorp/%s/latest/docs/guides/%s-upgrade-guide", 
			config.Provider, strings.TrimPrefix(config.ToVersion, "v")),
	}
}

func createDefaultTestTemplateData(config GeneratorConfig) TestTemplateData {
	return TestTemplateData{
		Provider:               config.Provider,
		ProviderTitle:          strings.Title(config.Provider),
		FromVersion:            config.FromVersion,
		ToVersion:              config.ToVersion,
		RulePrefix:             fmt.Sprintf("tfug.%s", config.Provider),
		ExpectedBasicFindings:  3,
		ExpectedRuleIDs: []string{
			fmt.Sprintf("tfug.%s.provider.%s_to_%s.min_version.001", config.Provider, config.FromVersion, config.ToVersion),
		},
	}
}

// Interactive data collection functions (simplified for space)
func collectProviderTemplateData(scanner *bufio.Scanner, provider, fromVersion, toVersion string) ProviderTemplateData {
	// This would collect interactive input for provider template data
	return createDefaultProviderTemplateData(GeneratorConfig{
		Provider:    provider,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	})
}

func collectModuleTemplateData(scanner *bufio.Scanner, provider, fromVersion, toVersion string) ModuleTemplateData {
	// This would collect interactive input for module template data
	return createDefaultModuleTemplateData(GeneratorConfig{
		Provider:    provider,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	})
}

func collectRulesTemplateData(scanner *bufio.Scanner, provider, fromVersion, toVersion string) RulesTemplateData {
	// This would collect interactive input for rules template data
	return createDefaultRulesTemplateData(GeneratorConfig{
		Provider:    provider,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	})
}

func collectTestTemplateData(scanner *bufio.Scanner, provider, fromVersion, toVersion string) TestTemplateData {
	// This would collect interactive input for test template data
	return createDefaultTestTemplateData(GeneratorConfig{
		Provider:    provider,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	})
}