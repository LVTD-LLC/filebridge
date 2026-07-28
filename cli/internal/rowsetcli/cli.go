package rowsetcli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultAPIBase = "https://rowset.lvtd.dev/api/"

var Version = "dev"

const (
	maxCollectionPageSize = 100
	maxSearchResults      = 50
	maxPreviewPageSize    = 100
	maxPreviewPassword    = 4096
	maxImageBytes         = 8 * 1024 * 1024
	maxAudioBytes         = 32 * 1024 * 1024
	maxRequestFileBytes   = 16 * 1024 * 1024
)

type IO struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	HTTPClient *http.Client
}

type config struct {
	apiBase   string
	apiKeyEnv string
	compact   bool
}

type requestOptions struct {
	auth       bool
	body       any
	bodyBytes  []byte
	outputPath string
	rawOutput  bool
}

type repeatedStrings []string

type apiErrorResponse struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail"`
	UpgradeURL string `json:"upgrade_url"`
}

func (values *repeatedStrings) String() string {
	return strings.Join(*values, ",")
}

func (values *repeatedStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func Run(ctx context.Context, streams IO, args []string) error {
	if streams.Stdin == nil {
		streams.Stdin = os.Stdin
	}
	if streams.Stdout == nil {
		streams.Stdout = os.Stdout
	}
	if streams.Stderr == nil {
		streams.Stderr = os.Stderr
	}

	cfg := config{
		apiBase:   envOrDefault("ROWSET_API_BASE", defaultAPIBase),
		apiKeyEnv: "ROWSET_API_KEY",
	}

	global := flag.NewFlagSet("rowset", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	global.StringVar(&cfg.apiBase, "api-base", cfg.apiBase, "Rowset REST API base URL")
	global.StringVar(&cfg.apiKeyEnv, "api-key-env", cfg.apiKeyEnv, "environment variable containing the Rowset API key")
	global.BoolVar(&cfg.compact, "compact", false, "write JSON responses on one line")
	showHelp := global.Bool("help", false, "show help")
	showVersion := global.Bool("version", false, "show version")
	if err := global.Parse(args); err != nil {
		return wrapUsageError(err)
	}
	if *showVersion {
		_, err := fmt.Fprintf(streams.Stdout, "rowset %s\n", Version)
		return err
	}
	if *showHelp || len(global.Args()) == 0 {
		return printHelp(streams.Stdout)
	}
	if helpArgs, ok := requestedHelp(global.Args()); ok {
		return printCommandHelp(streams.Stdout, helpArgs)
	}

	return dispatch(ctx, streams, cfg, global.Args())
}

func dispatch(ctx context.Context, streams IO, cfg config, args []string) error {
	switch args[0] {
	case "capabilities":
		return runCapabilities(ctx, streams, cfg, args[1:])
	case "healthcheck":
		if len(args) != 1 {
			return usageError("usage: rowset healthcheck")
		}
		return doRequest(ctx, streams, cfg, http.MethodGet, "/healthcheck", nil, requestOptions{})
	case "user":
		return runUser(ctx, streams, cfg, args[1:])
	case "feedback":
		return runFeedback(ctx, streams, cfg, args[1:])
	case "api-key", "apikey":
		return runAPIKey(ctx, streams, cfg, args[1:])
	case "project":
		return runProject(ctx, streams, cfg, args[1:])
	case "dataset":
		return runDataset(ctx, streams, cfg, args[1:])
	case "preview":
		return runPreview(ctx, streams, cfg, args[1:])
	case "column":
		return runColumn(ctx, streams, cfg, args[1:])
	case "relationship":
		return runRelationship(ctx, streams, cfg, args[1:])
	case "row":
		return runRow(ctx, streams, cfg, args[1:])
	case "asset":
		return runAsset(ctx, streams, cfg, args[1:])
	case "export":
		return runExport(ctx, streams, cfg, args[1:])
	case "request":
		return runRawRequest(ctx, streams, cfg, args[1:])
	case "help":
		return printCommandHelp(streams.Stdout, args[1:])
	default:
		return usageErrorf("unknown command %q", args[0])
	}
}

func runCapabilities(ctx context.Context, streams IO, cfg config, args []string) error {
	fs := newFlagSet("capabilities")
	var topicValues repeatedStrings
	fs.Var(&topicValues, "topic", "capability topic (repeatable or comma-separated)")
	includeUseCases := fs.Bool("include-use-cases", false, "include relevant use cases")
	full := fs.Bool("full", false, "return the complete capability guide")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if len(topicValues) > 0 && *full {
		return usageError("--topic cannot be combined with --full")
	}

	topics := make([]string, 0, len(topicValues))
	for _, value := range topicValues {
		topics = append(topics, splitCSV(value)...)
	}
	values := url.Values{}
	addQuery(values, "topics", strings.Join(topics, ","))
	if *includeUseCases {
		values.Set("include_use_cases", "true")
	}
	if *full {
		values.Set("full", "true")
	}
	return doRequest(ctx, streams, cfg, http.MethodGet, "/capabilities", values, requestOptions{})
}

func runUser(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) != 1 || args[0] != "info" {
		return usageError("usage: rowset user info")
	}
	return doRequest(ctx, streams, cfg, http.MethodGet, "/user", nil, requestOptions{auth: true})
}

func runFeedback(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 || args[0] != "submit" {
		return usageError("usage: rowset feedback submit --feedback TEXT [--page PATH] [--context JSON]")
	}
	fs := newFlagSet("feedback submit")
	feedback := fs.String("feedback", "", "feedback text")
	page := fs.String("page", "", "page or context path")
	contextJSON := fs.String("context", "", "JSON object with feedback context")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if *feedback == "" {
		return usageError("--feedback is required")
	}
	body := map[string]any{"feedback": *feedback}
	if flagWasSet(fs, "page") {
		body["page"] = *page
	}
	if flagWasSet(fs, "context") {
		context, err := parseJSONObject(*contextJSON, "--context")
		if err != nil {
			return err
		}
		body["context"] = context
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, "/feedback", nil, requestOptions{
		auth: true,
		body: body,
	})
}

func runAPIKey(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return usageError("usage: rowset api-key create --name NAME [--access-level read|read_write|admin]")
	}
	fs := newFlagSet("api-key create")
	name := fs.String("name", "", "API key name")
	accessLevel := fs.String("access-level", "read_write", "access level")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if *name == "" {
		return usageError("--name is required")
	}
	switch *accessLevel {
	case "read", "read_write", "admin":
	default:
		return usageError("--access-level must be read, read_write, or admin")
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, "/agent-api-keys", nil, requestOptions{
		auth: true,
		body: map[string]any{"name": *name, "access_level": *accessLevel},
	})
}

func runProject(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 {
		return usageError("usage: rowset project <list|search|create|get|update|metadata|archive|section>")
	}
	switch args[0] {
	case "list":
		fs := newFlagSet("project list")
		limit, offset := paginationFlags(fs, 100, 0)
		query := fs.String("query", "", "project search query")
		if err := parsePaginationFlags(fs, args[1:], limit, offset); err != nil {
			return err
		}
		values := paginationValues(*limit, *offset)
		addQuery(values, "query", *query)
		return doRequest(ctx, streams, cfg, http.MethodGet, "/projects", values, requestOptions{auth: true})
	case "search":
		if len(args) < 2 {
			return usageError("usage: rowset project search QUERY [--limit N] [--offset N]")
		}
		fs := newFlagSet("project search")
		limit, offset := paginationFlags(fs, 100, 0)
		if err := parsePaginationFlags(fs, args[2:], limit, offset); err != nil {
			return err
		}
		values := paginationValues(*limit, *offset)
		values.Set("query", args[1])
		return doRequest(ctx, streams, cfg, http.MethodGet, "/projects", values, requestOptions{auth: true})
	case "create":
		return createProject(ctx, streams, cfg, args[1:])
	case "get":
		if len(args) < 2 {
			return usageError("usage: rowset project get PROJECT_KEY [--limit N] [--offset N]")
		}
		fs := newFlagSet("project get")
		limit, offset := paginationFlags(fs, 100, 0)
		if err := parsePaginationFlags(fs, args[2:], limit, offset); err != nil {
			return err
		}
		return doRequest(
			ctx,
			streams,
			cfg,
			http.MethodGet,
			apiPath("projects", args[1]),
			paginationValues(*limit, *offset),
			requestOptions{auth: true},
		)
	case "update":
		return updateProject(ctx, streams, cfg, args[1:])
	case "metadata":
		return updateProjectMetadata(ctx, streams, cfg, args[1:])
	case "archive":
		if len(args) != 2 {
			return usageError("usage: rowset project archive PROJECT_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodDelete, apiPath("projects", args[1]), nil, requestOptions{auth: true})
	case "section":
		return runProjectSection(ctx, streams, cfg, args[1:])
	default:
		return usageErrorf("unknown project command %q", args[0])
	}
}

func createProject(ctx context.Context, streams IO, cfg config, args []string) error {
	fs := newFlagSet("project create")
	name := fs.String("name", "", "project name")
	description := fs.String("description", "", "project description")
	metadataJSON := fs.String("metadata", "", "project metadata JSON object")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *name == "" {
		return usageError("--name is required")
	}
	body := map[string]any{"name": *name}
	if flagWasSet(fs, "description") {
		body["description"] = *description
	}
	if flagWasSet(fs, "metadata") {
		metadata, err := parseJSONObject(*metadataJSON, "--metadata")
		if err != nil {
			return err
		}
		body["metadata"] = metadata
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, "/projects", nil, requestOptions{auth: true, body: body})
}

func updateProject(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset project update PROJECT_KEY [--name NAME] [--description TEXT]")
	}
	fs := newFlagSet("project update")
	name := fs.String("name", "", "project name")
	description := fs.String("description", "", "project description")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	body := map[string]any{}
	if flagWasSet(fs, "name") {
		body["name"] = *name
	}
	if flagWasSet(fs, "description") {
		body["description"] = *description
	}
	if len(body) == 0 {
		return usageError("at least one of --name or --description is required")
	}
	return doRequest(ctx, streams, cfg, http.MethodPatch, apiPath("projects", args[0]), nil, requestOptions{auth: true, body: body})
}

func updateProjectMetadata(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset project metadata PROJECT_KEY --metadata JSON")
	}
	fs := newFlagSet("project metadata")
	metadataJSON := fs.String("metadata", "", "project metadata JSON object")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if !flagWasSet(fs, "metadata") {
		return usageError("--metadata is required")
	}
	metadata, err := parseJSONObject(*metadataJSON, "--metadata")
	if err != nil {
		return err
	}
	return doRequest(ctx, streams, cfg, http.MethodPatch, apiPath("projects", args[0], "metadata"), nil, requestOptions{
		auth: true,
		body: map[string]any{"metadata": metadata},
	})
}

func runProjectSection(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 {
		return usageError("usage: rowset project section <list|create|update|archive>")
	}
	switch args[0] {
	case "list":
		if len(args) < 2 {
			return usageError("usage: rowset project section list PROJECT_KEY [--limit N] [--offset N]")
		}
		fs := newFlagSet("project section list")
		limit, offset := paginationFlags(fs, 100, 0)
		if err := parsePaginationFlags(fs, args[2:], limit, offset); err != nil {
			return err
		}
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("projects", args[1], "sections"), paginationValues(*limit, *offset), requestOptions{auth: true})
	case "create":
		if len(args) < 2 {
			return usageError("usage: rowset project section create PROJECT_KEY --name NAME")
		}
		return createProjectSection(ctx, streams, cfg, args[1], args[2:])
	case "update":
		if len(args) < 3 {
			return usageError("usage: rowset project section update PROJECT_KEY SECTION_KEY [--name NAME] [--description TEXT]")
		}
		return updateProjectSection(ctx, streams, cfg, args[1], args[2], args[3:])
	case "archive":
		if len(args) != 3 {
			return usageError("usage: rowset project section archive PROJECT_KEY SECTION_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodDelete, apiPath("projects", args[1], "sections", args[2]), nil, requestOptions{auth: true})
	default:
		return usageErrorf("unknown project section command %q", args[0])
	}
}

func createProjectSection(ctx context.Context, streams IO, cfg config, projectKey string, args []string) error {
	fs := newFlagSet("project section create")
	name := fs.String("name", "", "section name")
	description := fs.String("description", "", "section description")
	metadataJSON := fs.String("metadata", "", "section metadata JSON object")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *name == "" {
		return usageError("--name is required")
	}
	body := map[string]any{"name": *name}
	if flagWasSet(fs, "description") {
		body["description"] = *description
	}
	if flagWasSet(fs, "metadata") {
		metadata, err := parseJSONObject(*metadataJSON, "--metadata")
		if err != nil {
			return err
		}
		body["metadata"] = metadata
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("projects", projectKey, "sections"), nil, requestOptions{auth: true, body: body})
}

func updateProjectSection(ctx context.Context, streams IO, cfg config, projectKey string, sectionKey string, args []string) error {
	fs := newFlagSet("project section update")
	name := fs.String("name", "", "section name")
	description := fs.String("description", "", "section description")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	body := map[string]any{}
	if flagWasSet(fs, "name") {
		body["name"] = *name
	}
	if flagWasSet(fs, "description") {
		body["description"] = *description
	}
	if len(body) == 0 {
		return usageError("at least one of --name or --description is required")
	}
	return doRequest(ctx, streams, cfg, http.MethodPatch, apiPath("projects", projectKey, "sections", sectionKey), nil, requestOptions{auth: true, body: body})
}

func runDataset(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 {
		return usageError("usage: rowset dataset <list|search|archived|get|create|metadata|column-types|project|archive|restore>")
	}
	switch args[0] {
	case "list":
		return listDatasets(ctx, streams, cfg, args[1:])
	case "search":
		if len(args) < 2 {
			return usageError("usage: rowset dataset search QUERY [filters]")
		}
		return listDatasets(ctx, streams, cfg, append([]string{"--query", args[1]}, args[2:]...))
	case "archived":
		fs := newFlagSet("dataset archived")
		limit, offset := paginationFlags(fs, 100, 0)
		if err := parsePaginationFlags(fs, args[1:], limit, offset); err != nil {
			return err
		}
		return doRequest(ctx, streams, cfg, http.MethodGet, "/datasets/archived", paginationValues(*limit, *offset), requestOptions{auth: true})
	case "get":
		if len(args) != 2 {
			return usageError("usage: rowset dataset get DATASET_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[1]), nil, requestOptions{auth: true})
	case "create":
		return createDataset(ctx, streams, cfg, args[1:])
	case "metadata":
		return updateDatasetMetadata(ctx, streams, cfg, args[1:])
	case "column-types":
		return updateDatasetColumnTypes(ctx, streams, cfg, args[1:])
	case "project":
		return updateDatasetProject(ctx, streams, cfg, args[1:])
	case "archive":
		if len(args) != 2 {
			return usageError("usage: rowset dataset archive DATASET_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodDelete, apiPath("datasets", args[1]), nil, requestOptions{auth: true})
	case "restore":
		if len(args) != 2 {
			return usageError("usage: rowset dataset restore DATASET_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[1], "restore"), nil, requestOptions{auth: true})
	default:
		return usageErrorf("unknown dataset command %q", args[0])
	}
}

func listDatasets(ctx context.Context, streams IO, cfg config, args []string) error {
	fs := newFlagSet("dataset list")
	limit, offset := paginationFlags(fs, 100, 0)
	query := fs.String("query", "", "dataset search query")
	projectKey := fs.String("project-key", "", "project key")
	sectionKey := fs.String("section-key", "", "project section key")
	headerContains := fs.String("header-contains", "", "exact header name")
	status := fs.String("status", "", "dataset status")
	updatedAfter := fs.String("updated-after", "", "ISO date or datetime")
	if err := parsePaginationFlags(fs, args, limit, offset); err != nil {
		return err
	}
	values := paginationValues(*limit, *offset)
	addQuery(values, "query", *query)
	addQuery(values, "project_key", *projectKey)
	addQuery(values, "section_key", *sectionKey)
	addQuery(values, "header_contains", *headerContains)
	addQuery(values, "status", *status)
	addQuery(values, "updated_after", *updatedAfter)
	return doRequest(ctx, streams, cfg, http.MethodGet, "/datasets", values, requestOptions{auth: true})
}

func createDataset(ctx context.Context, streams IO, cfg config, args []string) error {
	fs := newFlagSet("dataset create")
	name := fs.String("name", "", "dataset name")
	description := fs.String("description", "", "dataset description")
	instructions := fs.String("instructions", "", "persistent agent instructions")
	metadataJSON := fs.String("metadata", "", "dataset metadata JSON object")
	headers := fs.String("headers", "", "comma-separated headers")
	indexColumn := fs.String("index-column", "", "index column")
	columnTypesJSON := fs.String("column-types", "", "column type JSON object")
	projectKey := fs.String("project-key", "", "project key")
	sectionKey := fs.String("section-key", "", "section key")
	rowsJSON := fs.String("rows", "", "JSON array of rows")
	var rowValues repeatedStrings
	fs.Var(&rowValues, "row", "JSON object row; may be repeated")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *name == "" {
		return usageError("--name is required")
	}
	body := map[string]any{"name": *name}
	if flagWasSet(fs, "description") {
		body["description"] = *description
	}
	if flagWasSet(fs, "instructions") {
		body["instructions"] = *instructions
	}
	if flagWasSet(fs, "metadata") {
		metadata, err := parseJSONObject(*metadataJSON, "--metadata")
		if err != nil {
			return err
		}
		body["metadata"] = metadata
	}
	if flagWasSet(fs, "headers") {
		body["headers"] = splitCSV(*headers)
	}
	if flagWasSet(fs, "index-column") {
		body["index_column"] = *indexColumn
	}
	if flagWasSet(fs, "column-types") {
		columnTypes, err := parseJSONObject(*columnTypesJSON, "--column-types")
		if err != nil {
			return err
		}
		body["column_types"] = columnTypes
	}
	if flagWasSet(fs, "project-key") {
		body["project_key"] = *projectKey
	}
	if flagWasSet(fs, "section-key") {
		body["section_key"] = *sectionKey
	}
	rows, err := parseRows(*rowsJSON, rowValues)
	if err != nil {
		return err
	}
	if rows != nil {
		body["rows"] = rows
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, "/datasets", nil, requestOptions{auth: true, body: body})
}

func updateDatasetMetadata(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset dataset metadata DATASET_KEY [--description TEXT] [--instructions TEXT] [--metadata JSON]")
	}
	fs := newFlagSet("dataset metadata")
	description := fs.String("description", "", "dataset description")
	instructions := fs.String("instructions", "", "dataset instructions")
	metadataJSON := fs.String("metadata", "", "dataset metadata JSON object")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	body := map[string]any{}
	if flagWasSet(fs, "description") {
		body["description"] = *description
	}
	if flagWasSet(fs, "instructions") {
		body["instructions"] = *instructions
	}
	if flagWasSet(fs, "metadata") {
		metadata, err := parseJSONObject(*metadataJSON, "--metadata")
		if err != nil {
			return err
		}
		body["metadata"] = metadata
	}
	if len(body) == 0 {
		return usageError("at least one metadata field is required")
	}
	return doRequest(ctx, streams, cfg, http.MethodPatch, apiPath("datasets", args[0], "metadata"), nil, requestOptions{auth: true, body: body})
}

func updateDatasetColumnTypes(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset dataset column-types DATASET_KEY --column-types JSON")
	}
	fs := newFlagSet("dataset column-types")
	columnTypesJSON := fs.String("column-types", "", "column type JSON object")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if !flagWasSet(fs, "column-types") {
		return usageError("--column-types is required")
	}
	columnTypes, err := parseJSONObject(*columnTypesJSON, "--column-types")
	if err != nil {
		return err
	}
	return doRequest(ctx, streams, cfg, http.MethodPatch, apiPath("datasets", args[0], "column-types"), nil, requestOptions{
		auth: true,
		body: map[string]any{"column_types": columnTypes},
	})
}

func updateDatasetProject(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset dataset project DATASET_KEY (--project-key KEY [--section-key KEY] | --clear)")
	}
	fs := newFlagSet("dataset project")
	projectKey := fs.String("project-key", "", "project key")
	sectionKey := fs.String("section-key", "", "section key")
	clearProject := fs.Bool("clear", false, "remove project assignment")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	body := map[string]any{}
	if *clearProject {
		if flagWasSet(fs, "project-key") || flagWasSet(fs, "section-key") {
			return usageError("--clear cannot be combined with --project-key or --section-key")
		}
		body["project_key"] = nil
	} else if flagWasSet(fs, "project-key") {
		body["project_key"] = *projectKey
	} else {
		return usageError("--project-key or --clear is required")
	}
	if flagWasSet(fs, "section-key") {
		body["section_key"] = *sectionKey
	}
	return doRequest(ctx, streams, cfg, http.MethodPatch, apiPath("datasets", args[0], "project"), nil, requestOptions{auth: true, body: body})
}

func runPreview(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 || args[0] != "update" || len(args) < 2 {
		return usageError(
			"usage: rowset preview update DATASET_KEY [--enabled true|false] " +
				"[--page-size N] [--password-stdin | --password-env NAME | --clear-password]",
		)
	}
	fs := newFlagSet("preview update")
	enabled := fs.String("enabled", "", "true or false")
	pageSize := fs.Int("page-size", 0, "public preview page size")
	passwordStdin := fs.Bool("password-stdin", false, "read the public preview password from stdin")
	passwordEnv := fs.String(
		"password-env",
		"",
		"environment variable containing the public preview password",
	)
	clearPassword := fs.Bool("clear-password", false, "clear public preview password")
	if err := parseFlags(fs, args[2:]); err != nil {
		return err
	}
	body := map[string]any{}
	if flagWasSet(fs, "enabled") {
		value, err := strconv.ParseBool(*enabled)
		if err != nil {
			return usageErrorf("--enabled must be true or false: %v", err)
		}
		body["public_enabled"] = value
	}
	if flagWasSet(fs, "page-size") {
		if err := validateIntRange("--page-size", *pageSize, 1, maxPreviewPageSize); err != nil {
			return err
		}
		body["public_page_size"] = *pageSize
	}
	passwordSources := 0
	if *passwordStdin {
		passwordSources++
	}
	if flagWasSet(fs, "password-env") {
		passwordSources++
	}
	if *clearPassword {
		passwordSources++
	}
	if passwordSources > 1 {
		return usageError(
			"use only one of --password-stdin, --password-env, or --clear-password",
		)
	}
	if *passwordStdin {
		password, err := readSecret(streams.Stdin, maxPreviewPassword)
		if err != nil {
			return err
		}
		body["public_password"] = password
	}
	if flagWasSet(fs, "password-env") {
		if strings.TrimSpace(*passwordEnv) == "" {
			return usageError("--password-env requires a non-empty environment variable name")
		}
		password, ok := os.LookupEnv(*passwordEnv)
		if !ok || password == "" {
			return usageErrorf("%s must contain the public preview password", *passwordEnv)
		}
		body["public_password"] = password
	}
	if *clearPassword {
		body["clear_public_password"] = true
	}
	if len(body) == 0 {
		return usageError("at least one preview setting is required")
	}
	return doRequest(ctx, streams, cfg, http.MethodPatch, apiPath("datasets", args[1], "public-preview"), nil, requestOptions{auth: true, body: body})
}

func runColumn(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 {
		return usageError("usage: rowset column <add|rename|drop|reorder>")
	}
	switch args[0] {
	case "add":
		return addColumn(ctx, streams, cfg, args[1:])
	case "rename":
		if len(args) != 4 {
			return usageError("usage: rowset column rename DATASET_KEY OLD_NAME NEW_NAME")
		}
		return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[1], "columns", "rename"), nil, requestOptions{
			auth: true,
			body: map[string]any{"old_name": args[2], "new_name": args[3]},
		})
	case "drop":
		if len(args) != 3 {
			return usageError("usage: rowset column drop DATASET_KEY NAME")
		}
		return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[1], "columns", "drop"), nil, requestOptions{
			auth: true,
			body: map[string]any{"name": args[2]},
		})
	case "reorder":
		if len(args) < 2 {
			return usageError("usage: rowset column reorder DATASET_KEY --headers a,b,c")
		}
		fs := newFlagSet("column reorder")
		headers := fs.String("headers", "", "comma-separated headers")
		if err := parseFlags(fs, args[2:]); err != nil {
			return err
		}
		if !flagWasSet(fs, "headers") {
			return usageError("--headers is required")
		}
		return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[1], "columns", "reorder"), nil, requestOptions{
			auth: true,
			body: map[string]any{"headers": splitCSV(*headers)},
		})
	default:
		return usageErrorf("unknown column command %q", args[0])
	}
}

func addColumn(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset column add DATASET_KEY --name NAME")
	}
	fs := newFlagSet("column add")
	name := fs.String("name", "", "column name")
	defaultValue := fs.String("default-value", "", "default string value")
	defaultJSON := fs.String("default-json", "", "default JSON value")
	columnType := fs.String("column-type", "", "column type string or JSON object")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if *name == "" {
		return usageError("--name is required")
	}
	if flagWasSet(fs, "default-json") && flagWasSet(fs, "default-value") {
		return usageError("use only one of --default-json or --default-value")
	}
	body := map[string]any{"name": *name}
	if flagWasSet(fs, "default-json") {
		value, err := parseJSONValue(*defaultJSON, "--default-json")
		if err != nil {
			return err
		}
		body["default_value"] = value
	} else if flagWasSet(fs, "default-value") {
		body["default_value"] = *defaultValue
	}
	if flagWasSet(fs, "column-type") {
		value, err := parseMaybeJSON(*columnType, "--column-type")
		if err != nil {
			return err
		}
		body["column_type"] = value
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[0], "columns"), nil, requestOptions{auth: true, body: body})
}

func runRelationship(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 {
		return usageError("usage: rowset relationship <list|create|resolve|delete>")
	}
	switch args[0] {
	case "list":
		if len(args) != 2 {
			return usageError("usage: rowset relationship list DATASET_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[1], "relationships"), nil, requestOptions{auth: true})
	case "create":
		return createRelationship(ctx, streams, cfg, args[1:])
	case "resolve":
		if len(args) < 3 {
			return usageError("usage: rowset relationship resolve DATASET_KEY RELATIONSHIP_KEY --source-index-value VALUE")
		}
		fs := newFlagSet("relationship resolve")
		sourceIndexValue := fs.String("source-index-value", "", "source row index value")
		if err := parseFlags(fs, args[3:]); err != nil {
			return err
		}
		if *sourceIndexValue == "" {
			return usageError("--source-index-value is required")
		}
		values := url.Values{}
		values.Set("source_index_value", *sourceIndexValue)
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[1], "relationships", args[2], "resolve"), values, requestOptions{auth: true})
	case "delete":
		if len(args) != 3 {
			return usageError("usage: rowset relationship delete DATASET_KEY RELATIONSHIP_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodDelete, apiPath("datasets", args[1], "relationships", args[2]), nil, requestOptions{auth: true})
	default:
		return usageErrorf("unknown relationship command %q", args[0])
	}
}

func createRelationship(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset relationship create DATASET_KEY --source-column COLUMN --target-dataset-key KEY")
	}
	fs := newFlagSet("relationship create")
	sourceColumn := fs.String("source-column", "", "source column")
	targetDatasetKey := fs.String("target-dataset-key", "", "target dataset key")
	name := fs.String("name", "", "relationship name")
	enforceIntegrity := fs.String("enforce-integrity", "true", "true or false")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if *sourceColumn == "" || *targetDatasetKey == "" {
		return usageError("--source-column and --target-dataset-key are required")
	}
	enforce, err := strconv.ParseBool(*enforceIntegrity)
	if err != nil {
		return usageErrorf("--enforce-integrity must be true or false: %v", err)
	}
	body := map[string]any{
		"source_column":      *sourceColumn,
		"target_dataset_key": *targetDatasetKey,
		"enforce_integrity":  enforce,
	}
	if flagWasSet(fs, "name") {
		body["name"] = *name
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[0], "relationships"), nil, requestOptions{auth: true, body: body})
}

func runRow(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 {
		return usageError("usage: rowset row <list|search|search-dataset|get|get-by-index|create|update|update-by-index|delete>")
	}
	switch args[0] {
	case "list":
		return listRows(ctx, streams, cfg, args[1:])
	case "search":
		return searchRows(ctx, streams, cfg, args[1:])
	case "search-dataset":
		return searchDatasetRows(ctx, streams, cfg, args[1:])
	case "get":
		if len(args) != 3 {
			return usageError("usage: rowset row get DATASET_KEY ROW_ID")
		}
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[1], "rows", args[2]), nil, requestOptions{auth: true})
	case "get-by-index":
		if len(args) != 3 {
			return usageError("usage: rowset row get-by-index DATASET_KEY INDEX_VALUE")
		}
		values := url.Values{}
		values.Set("index_value", args[2])
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[1], "rows", "by-index"), values, requestOptions{auth: true})
	case "create":
		if len(args) < 2 {
			return usageError("usage: rowset row create DATASET_KEY --data JSON")
		}
		return rowWrite(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[1], "rows"), nil, args[2:])
	case "update":
		if len(args) < 3 {
			return usageError("usage: rowset row update DATASET_KEY ROW_ID --data JSON")
		}
		return rowWrite(ctx, streams, cfg, http.MethodPatch, apiPath("datasets", args[1], "rows", args[2]), nil, args[3:])
	case "update-by-index":
		if len(args) < 3 {
			return usageError("usage: rowset row update-by-index DATASET_KEY INDEX_VALUE --data JSON")
		}
		values := url.Values{}
		values.Set("index_value", args[2])
		return rowWrite(ctx, streams, cfg, http.MethodPatch, apiPath("datasets", args[1], "rows", "by-index"), values, args[3:])
	case "delete":
		if len(args) != 3 {
			return usageError("usage: rowset row delete DATASET_KEY ROW_ID")
		}
		return doRequest(ctx, streams, cfg, http.MethodDelete, apiPath("datasets", args[1], "rows", args[2]), nil, requestOptions{auth: true})
	default:
		return usageErrorf("unknown row command %q", args[0])
	}
}

func listRows(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset row list DATASET_KEY [--limit N] [--filters JSON]")
	}
	fs := newFlagSet("row list")
	limit, offset := paginationFlags(fs, 100, 0)
	query := fs.String("query", "", "row query")
	filters := fs.String("filters", "", "row filters JSON object")
	sort := fs.String("sort", "", "sort header")
	direction := fs.String("direction", "", "asc or desc")
	if err := parsePaginationFlags(fs, args[1:], limit, offset); err != nil {
		return err
	}
	values := paginationValues(*limit, *offset)
	addQuery(values, "query", *query)
	addQuery(values, "filters", *filters)
	addQuery(values, "sort", *sort)
	addQuery(values, "direction", *direction)
	return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[0], "rows"), values, requestOptions{auth: true})
}

func searchRows(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset row search QUERY [filters]")
	}
	fs := newFlagSet("row search")
	filtersJSON := fs.String("filters", "", "row filters JSON object")
	filterOperatorsJSON := fs.String("filter-operators", "", "filter operators JSON object")
	datasetKey := fs.String("dataset-key", "", "dataset key")
	projectKey := fs.String("project-key", "", "project key")
	sectionKey := fs.String("section-key", "", "section key")
	status := fs.String("status", "", "dataset status")
	archived := fs.String("archived", "", "true, false, or null")
	sort := fs.String("sort", "", "rank, dataset, or row_number")
	direction := fs.String("direction", "", "asc or desc")
	limit := fs.Int("limit", 10, "result limit")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if err := validateIntRange("--limit", *limit, 1, maxSearchResults); err != nil {
		return err
	}
	body := map[string]any{"query": args[0]}
	if flagWasSet(fs, "filters") {
		value, err := parseJSONObject(*filtersJSON, "--filters")
		if err != nil {
			return err
		}
		body["filters"] = value
	}
	if flagWasSet(fs, "filter-operators") {
		value, err := parseJSONObject(*filterOperatorsJSON, "--filter-operators")
		if err != nil {
			return err
		}
		body["filter_operators"] = value
	}
	if flagWasSet(fs, "dataset-key") {
		body["dataset_key"] = *datasetKey
	}
	if flagWasSet(fs, "project-key") {
		body["project_key"] = *projectKey
	}
	if flagWasSet(fs, "section-key") {
		body["section_key"] = *sectionKey
	}
	if flagWasSet(fs, "status") {
		body["status"] = *status
	}
	if flagWasSet(fs, "archived") {
		if strings.EqualFold(*archived, "null") {
			body["archived"] = nil
		} else {
			value, err := strconv.ParseBool(*archived)
			if err != nil {
				return usageErrorf("--archived must be true, false, or null: %v", err)
			}
			body["archived"] = value
		}
	}
	if flagWasSet(fs, "sort") {
		body["sort"] = *sort
	}
	if flagWasSet(fs, "direction") {
		body["direction"] = *direction
	}
	if flagWasSet(fs, "limit") {
		body["limit"] = *limit
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, "/search", nil, requestOptions{auth: true, body: body})
}

func searchDatasetRows(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 2 {
		return usageError("usage: rowset row search-dataset DATASET_KEY QUERY [--filters JSON] [--limit N]")
	}
	fs := newFlagSet("row search-dataset")
	filtersJSON := fs.String("filters", "", "row filters JSON object")
	limit := fs.Int("limit", 10, "result limit")
	if err := parseFlags(fs, args[2:]); err != nil {
		return err
	}
	if err := validateIntRange("--limit", *limit, 1, maxSearchResults); err != nil {
		return err
	}
	body := map[string]any{"query": args[1]}
	if flagWasSet(fs, "filters") {
		value, err := parseJSONObject(*filtersJSON, "--filters")
		if err != nil {
			return err
		}
		body["filters"] = value
	}
	if flagWasSet(fs, "limit") {
		body["limit"] = *limit
	}
	return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[0], "search"), nil, requestOptions{auth: true, body: body})
}

func rowWrite(ctx context.Context, streams IO, cfg config, method string, path string, query url.Values, args []string) error {
	fs := newFlagSet("row write")
	dataJSON := fs.String("data", "", "row data JSON object")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if !flagWasSet(fs, "data") {
		return usageError("--data is required")
	}
	data, err := parseJSONObject(*dataJSON, "--data")
	if err != nil {
		return err
	}
	return doRequest(ctx, streams, cfg, method, path, query, requestOptions{
		auth: true,
		body: map[string]any{"data": data},
	})
}

func runAsset(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) == 0 {
		return usageError("usage: rowset asset <attach|get|content>")
	}
	switch args[0] {
	case "attach":
		return attachAsset(ctx, streams, cfg, args[1:])
	case "get":
		if len(args) != 3 {
			return usageError("usage: rowset asset get DATASET_KEY ASSET_KEY")
		}
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[1], "assets", args[2]), nil, requestOptions{auth: true})
	case "content":
		if len(args) < 3 {
			return usageError("usage: rowset asset content DATASET_KEY ASSET_KEY [--variant original|thumbnail] [--output PATH]")
		}
		fs := newFlagSet("asset content")
		variant := fs.String("variant", "original", "asset variant")
		output := fs.String("output", "", "output path")
		if err := parseFlags(fs, args[3:]); err != nil {
			return err
		}
		values := url.Values{}
		values.Set("variant", *variant)
		return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[1], "assets", args[2], "content"), values, requestOptions{
			auth:       true,
			outputPath: *output,
			rawOutput:  true,
		})
	default:
		return usageErrorf("unknown asset command %q", args[0])
	}
}

func attachAsset(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 1 {
		return usageError("usage: rowset asset attach DATASET_KEY --column COLUMN --file PATH (--row-id ID | --index-value VALUE) [--asset-type image|audio]")
	}
	fs := newFlagSet("asset attach")
	rowID := fs.String("row-id", "", "row id")
	indexValue := fs.String("index-value", "", "row index value")
	assetType := fs.String("asset-type", "image", "asset type: image or audio")
	column := fs.String("column", "", "asset column")
	filePath := fs.String("file", "", "asset file path")
	filename := fs.String("filename", "", "original filename")
	contentType := fs.String("content-type", "", "asset content type")
	if err := parseFlags(fs, args[1:]); err != nil {
		return err
	}
	if *column == "" || *filePath == "" {
		return usageError("--column and --file are required")
	}
	if (*rowID == "" && *indexValue == "") || (*rowID != "" && *indexValue != "") {
		return usageError("pass exactly one of --row-id or --index-value")
	}
	normalizedAssetType := strings.ToLower(strings.TrimSpace(*assetType))
	if normalizedAssetType != "image" && normalizedAssetType != "audio" {
		return usageError("--asset-type must be image or audio")
	}
	maxBytes := int64(maxImageBytes)
	if normalizedAssetType == "audio" {
		maxBytes = maxAudioBytes
	}
	data, err := readFileBounded(*filePath, maxBytes)
	if err != nil {
		return fmt.Errorf("read asset file: %w", err)
	}
	base64Field := normalizedAssetType + "_base64"
	body := map[string]any{
		"column_name": *column,
		base64Field:   base64.StdEncoding.EncodeToString(data),
	}
	if flagWasSet(fs, "filename") {
		body["filename"] = *filename
	} else {
		body["filename"] = filepath.Base(*filePath)
	}
	if flagWasSet(fs, "content-type") {
		body["content_type"] = *contentType
	}
	if *rowID != "" {
		return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[0], "rows", *rowID, normalizedAssetType), nil, requestOptions{auth: true, body: body})
	}
	values := url.Values{}
	values.Set("index_value", *indexValue)
	return doRequest(ctx, streams, cfg, http.MethodPost, apiPath("datasets", args[0], "rows", "by-index", normalizedAssetType), values, requestOptions{auth: true, body: body})
}

func runExport(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 2 {
		return usageError("usage: rowset export DATASET_KEY csv|jsonl|xlsx|sqlite [--output PATH]")
	}
	format := strings.TrimPrefix(strings.ToLower(args[1]), ".")
	switch format {
	case "csv", "jsonl", "xlsx", "sqlite":
	default:
		return usageErrorf("unsupported export format %q", args[1])
	}
	fs := newFlagSet("export")
	output := fs.String("output", "", "output path")
	if err := parseFlags(fs, args[2:]); err != nil {
		return err
	}
	return doRequest(ctx, streams, cfg, http.MethodGet, apiPath("datasets", args[0], "export."+format), nil, requestOptions{
		auth:       true,
		outputPath: *output,
		rawOutput:  true,
	})
}

func runRawRequest(ctx context.Context, streams IO, cfg config, args []string) error {
	if len(args) < 2 {
		return usageError("usage: rowset request METHOD PATH [--json JSON | --file PATH] [--output PATH] [--no-auth]")
	}
	method := strings.ToUpper(args[0])
	path := args[1]
	fs := newFlagSet("request")
	jsonBody := fs.String("json", "", "JSON request body")
	bodyFile := fs.String("file", "", "file containing request body")
	output := fs.String("output", "", "output path")
	noAuth := fs.Bool("no-auth", false, "do not send bearer auth")
	if err := parseFlags(fs, args[2:]); err != nil {
		return err
	}
	var bodyBytes []byte
	if flagWasSet(fs, "json") && flagWasSet(fs, "file") {
		return usageError("use only one of --json or --file")
	}
	if flagWasSet(fs, "json") {
		if !json.Valid([]byte(*jsonBody)) {
			return usageError("--json must be valid JSON")
		}
		bodyBytes = []byte(*jsonBody)
	}
	if flagWasSet(fs, "file") {
		data, err := readFileBounded(*bodyFile, maxRequestFileBytes)
		if err != nil {
			return fmt.Errorf("read request file: %w", err)
		}
		bodyBytes = data
	}
	return doRequest(ctx, streams, cfg, method, path, nil, requestOptions{
		auth:       !*noAuth,
		bodyBytes:  bodyBytes,
		outputPath: *output,
		rawOutput:  *output != "",
	})
}

func buildEndpoint(apiBase string, path string, query url.Values, allowAbsolute bool) (string, error) {
	if isAbsoluteHTTPURL(path) {
		if !allowAbsolute {
			return "", usageError("absolute request URLs require --no-auth")
		}
		parsed, err := url.Parse(path)
		if err != nil {
			return "", err
		}
		if query != nil {
			values := parsed.Query()
			for key, rawValues := range query {
				for _, value := range rawValues {
					values.Add(key, value)
				}
			}
			parsed.RawQuery = values.Encode()
		}
		return parsed.String(), nil
	}
	if strings.TrimSpace(apiBase) == "" {
		return "", usageError("ROWSET_API_BASE or --api-base is required")
	}
	base := strings.TrimRight(apiBase, "/")
	cleanPath := "/" + strings.TrimLeft(path, "/")
	parsed, err := url.Parse(base + cleanPath)
	if err != nil {
		return "", err
	}
	if query != nil {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), nil
}

func isAbsoluteHTTPURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.IsAbs() && (strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https"))
}

func apiPath(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		for _, segment := range strings.Split(part, "/") {
			if segment == "" {
				continue
			}
			escaped = append(escaped, url.PathEscape(segment))
		}
	}
	return "/" + strings.Join(escaped, "/")
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return wrapUsageError(err)
	}
	if fs.NArg() != 0 {
		return usageErrorf("unexpected argument %q", fs.Arg(0))
	}
	return nil
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func paginationFlags(fs *flag.FlagSet, defaultLimit int, defaultOffset int) (*int, *int) {
	limit := fs.Int("limit", defaultLimit, "page limit")
	offset := fs.Int("offset", defaultOffset, "page offset")
	return limit, offset
}

func paginationValues(limit int, offset int) url.Values {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	return values
}

func parsePaginationFlags(
	fs *flag.FlagSet,
	args []string,
	limit *int,
	offset *int,
) error {
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	return validatePagination(*limit, *offset)
}

func validatePagination(limit int, offset int) error {
	if err := validateIntRange("--limit", limit, 1, maxCollectionPageSize); err != nil {
		return err
	}
	if offset < 0 {
		return usageError("--offset must be at least 0")
	}
	return nil
}

func validateIntRange(name string, value int, minimum int, maximum int) error {
	if value < minimum || value > maximum {
		return usageErrorf("%s must be between %d and %d", name, minimum, maximum)
	}
	return nil
}

func addQuery(values url.Values, key string, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	wasSet := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			wasSet = true
		}
	})
	return wasSet
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func parseRows(rowsJSON string, rowValues repeatedStrings) ([]any, error) {
	var rows []any
	if rowsJSON != "" {
		var parsed []any
		if err := json.Unmarshal([]byte(rowsJSON), &parsed); err != nil {
			return nil, usageErrorf("--rows must be a JSON array: %v", err)
		}
		for index, row := range parsed {
			if _, ok := row.(map[string]any); !ok {
				return nil, usageErrorf("--rows item %d must be a JSON object", index+1)
			}
		}
		rows = append(rows, parsed...)
	}
	for _, raw := range rowValues {
		row, err := parseJSONObject(raw, "--row")
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	if rows == nil {
		return nil, nil
	}
	return rows, nil
}

func parseJSONObject(raw string, flagName string) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, usageErrorf("%s must be a JSON object: %v", flagName, err)
	}
	if value == nil {
		return nil, usageErrorf("%s must be a JSON object", flagName)
	}
	return value, nil
}

func parseJSONValue(raw string, flagName string) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, usageErrorf("%s must be valid JSON: %v", flagName, err)
	}
	return value, nil
}

func parseMaybeJSON(raw string, flagName string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, `"`) {
		return parseJSONValue(trimmed, flagName)
	}
	return raw, nil
}

func formatJSON(data []byte, compact bool) []byte {
	var formatted bytes.Buffer
	var err error
	if compact {
		err = json.Compact(&formatted, data)
	} else {
		err = json.Indent(&formatted, data, "", "  ")
	}
	if err != nil {
		if len(data) > 0 && data[len(data)-1] != '\n' {
			return append(data, '\n')
		}
		return data
	}
	formatted.WriteByte('\n')
	return formatted.Bytes()
}

func readSecret(reader io.Reader, limit int64) (string, error) {
	data, err := readBoundedInput(reader, limit, "password")
	if err != nil {
		return "", err
	}
	password := strings.TrimSuffix(string(data), "\n")
	password = strings.TrimSuffix(password, "\r")
	if password == "" {
		return "", usageError("public preview password cannot be empty")
	}
	return password, nil
}

func readFileBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect file: %w", err)
	}
	if info.Mode().IsRegular() && info.Size() > limit {
		return nil, usageErrorf("file exceeds %d bytes", limit)
	}
	data, err := readBoundedInput(file, limit, "file")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readBoundedInput(reader io.Reader, limit int64, label string) ([]byte, error) {
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	if int64(len(data)) > limit {
		return nil, usageErrorf("%s exceeds %d bytes", label, limit)
	}
	return data, nil
}
