package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type OpenAPIService struct{}

func NewOpenAPIService() *OpenAPIService {
	return &OpenAPIService{}
}

// Collection types matching the Nikode format
type NikodeCollection struct {
	Name                string           `json:"name"`
	Version             string           `json:"version"`
	Environments        []Environment    `json:"environments"`
	ActiveEnvironmentID string           `json:"activeEnvironmentId"`
	Items               []CollectionItem `json:"items"`
}

type Environment struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Variables []Variable `json:"variables"`
}

type Variable struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
	Secret  bool   `json:"secret,omitempty"`
}

type CollectionItem struct {
	ID      string           `json:"id"`
	Type    string           `json:"type"`
	Name    string           `json:"name"`
	Items   []CollectionItem `json:"items,omitempty"`
	Method  string           `json:"method,omitempty"`
	URL     string           `json:"url,omitempty"`
	Params  []KeyValue       `json:"params,omitempty"`
	Headers []KeyValue       `json:"headers,omitempty"`
	Body    *RequestBody     `json:"body,omitempty"`
	Scripts *Scripts         `json:"scripts,omitempty"`
	Docs    string           `json:"docs,omitempty"`
}

type KeyValue struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type RequestBody struct {
	Type    string     `json:"type"`
	Content string     `json:"content,omitempty"`
	Entries []KeyValue `json:"entries,omitempty"`
}

type Scripts struct {
	Pre  string `json:"pre"`
	Post string `json:"post"`
}

// ParseOpenAPI parses OpenAPI content (auto-detects JSON/YAML) and returns the spec
func (s *OpenAPIService) ParseOpenAPI(content []byte) (any, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	// Try parsing as JSON first
	spec, err := loader.LoadFromData(content)
	if err != nil {
		// Try YAML parsing by converting to JSON first
		var yamlData any
		if yamlErr := yaml.Unmarshal(content, &yamlData); yamlErr != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
		}
		jsonContent, jsonErr := json.Marshal(yamlData)
		if jsonErr != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON: %w", jsonErr)
		}
		spec, err = loader.LoadFromData(jsonContent)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
		}
	}

	return spec, nil
}

// ConvertToNikode converts an OpenAPI spec to Nikode collection format
func (s *OpenAPIService) ConvertToNikode(specInterface any) (json.RawMessage, error) {
	spec, ok := specInterface.(*openapi3.T)
	if !ok {
		return nil, fmt.Errorf("invalid spec type, expected *openapi3.T")
	}

	collection := NikodeCollection{
		Name:                s.extractTitle(spec),
		Version:             s.extractVersion(spec),
		Environments:        s.createEnvironments(spec),
		ActiveEnvironmentID: "env-default",
		Items:               s.convertPaths(spec),
	}

	data, err := json.Marshal(collection)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *OpenAPIService) extractTitle(spec *openapi3.T) string {
	if spec.Info != nil && spec.Info.Title != "" {
		return spec.Info.Title
	}
	return "Imported API"
}

func (s *OpenAPIService) extractVersion(spec *openapi3.T) string {
	if spec.Info != nil && spec.Info.Version != "" {
		return spec.Info.Version
	}
	return "1.0.0"
}

func (s *OpenAPIService) extractBaseURL(spec *openapi3.T) string {
	if len(spec.Servers) > 0 && spec.Servers[0].URL != "" {
		return spec.Servers[0].URL
	}
	return "http://localhost:3000"
}

func (s *OpenAPIService) createEnvironments(spec *openapi3.T) []Environment {
	return []Environment{
		{
			ID:   "env-default",
			Name: "Default",
			Variables: []Variable{
				{
					Key:     "baseUrl",
					Value:   s.extractBaseURL(spec),
					Enabled: true,
				},
			},
		},
	}
}

func (s *OpenAPIService) convertPaths(spec *openapi3.T) []CollectionItem {
	// Group operations by tag
	tagFolders := make(map[string]*CollectionItem)
	var rootItems []CollectionItem
	var tagOrder []string

	if spec.Paths == nil {
		return []CollectionItem{}
	}

	for pathStr, pathItem := range spec.Paths.Map() {
		operations := map[string]*openapi3.Operation{
			"GET":     pathItem.Get,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"PATCH":   pathItem.Patch,
			"DELETE":  pathItem.Delete,
			"HEAD":    pathItem.Head,
			"OPTIONS": pathItem.Options,
		}

		for method, op := range operations {
			if op == nil {
				continue
			}

			request := s.convertOperationToRequest(pathStr, method, op)

			// Get tag for organization
			tag := s.getFirstTag(op)
			if tag == "" {
				rootItems = append(rootItems, request)
			} else {
				folder, exists := tagFolders[tag]
				if !exists {
					folder = &CollectionItem{
						ID:    s.generateID("folder", tag),
						Type:  "folder",
						Name:  tag,
						Items: []CollectionItem{},
					}
					tagFolders[tag] = folder
					tagOrder = append(tagOrder, tag)
				}
				folder.Items = append(folder.Items, request)
			}
		}
	}

	// Build final items list: folders first, then root items
	var items []CollectionItem
	for _, tag := range tagOrder {
		items = append(items, *tagFolders[tag])
	}
	items = append(items, rootItems...)

	return items
}

func (s *OpenAPIService) convertOperationToRequest(pathStr, method string, op *openapi3.Operation) CollectionItem {
	// Convert path parameters from {param} to {{param}}
	url := "{{baseUrl}}" + s.convertPathParams(pathStr)

	request := CollectionItem{
		ID:      s.generateID("req", s.getOperationID(op, method, pathStr)),
		Type:    "request",
		Name:    s.getOperationName(op, method, pathStr),
		Method:  method,
		URL:     url,
		Params:  s.extractQueryParams(op),
		Headers: s.extractHeaders(op),
		Body:    s.convertRequestBody(op),
		Scripts: &Scripts{Pre: "", Post: ""},
		Docs:    s.getDescription(op),
	}

	return request
}

func (s *OpenAPIService) convertPathParams(path string) string {
	// Convert {param} to {{param}}
	re := regexp.MustCompile(`\{([^}]+)\}`)
	return re.ReplaceAllString(path, "{{$1}}")
}

func (s *OpenAPIService) getFirstTag(op *openapi3.Operation) string {
	if len(op.Tags) > 0 {
		return op.Tags[0]
	}
	return ""
}

func (s *OpenAPIService) getOperationID(op *openapi3.Operation, method, path string) string {
	if op.OperationID != "" {
		return op.OperationID
	}
	// Generate a slug from method and path
	slug := strings.ToLower(method) + "-" + s.slugify(path)
	return slug
}

func (s *OpenAPIService) getOperationName(op *openapi3.Operation, method, path string) string {
	if op.Summary != "" {
		return op.Summary
	}
	return method + " " + path
}

func (s *OpenAPIService) getDescription(op *openapi3.Operation) string {
	if op.Description != "" {
		return op.Description
	}
	return ""
}

func (s *OpenAPIService) extractQueryParams(op *openapi3.Operation) []KeyValue {
	var params []KeyValue
	for _, paramRef := range op.Parameters {
		param := paramRef.Value
		if param != nil && param.In == "query" {
			params = append(params, KeyValue{
				Key:     param.Name,
				Value:   s.getExampleValue(param.Schema),
				Enabled: param.Required,
			})
		}
	}
	return params
}

func (s *OpenAPIService) extractHeaders(op *openapi3.Operation) []KeyValue {
	var headers []KeyValue
	for _, paramRef := range op.Parameters {
		param := paramRef.Value
		if param != nil && param.In == "header" {
			headers = append(headers, KeyValue{
				Key:     param.Name,
				Value:   s.getExampleValue(param.Schema),
				Enabled: param.Required,
			})
		}
	}
	return headers
}

func (s *OpenAPIService) convertRequestBody(op *openapi3.Operation) *RequestBody {
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return &RequestBody{Type: "none"}
	}

	content := op.RequestBody.Value.Content
	if content == nil {
		return &RequestBody{Type: "none"}
	}

	// Check for JSON
	if mediaType, ok := content["application/json"]; ok {
		return &RequestBody{
			Type:    "json",
			Content: s.generateExampleJSON(mediaType.Schema),
		}
	}

	// Check for form-data
	if mediaType, ok := content["multipart/form-data"]; ok {
		return &RequestBody{
			Type:    "form-data",
			Entries: s.extractFormEntries(mediaType.Schema),
		}
	}

	// Check for urlencoded
	if mediaType, ok := content["application/x-www-form-urlencoded"]; ok {
		return &RequestBody{
			Type:    "x-www-form-urlencoded",
			Entries: s.extractFormEntries(mediaType.Schema),
		}
	}

	// Check for text/plain
	if _, ok := content["text/plain"]; ok {
		return &RequestBody{
			Type:    "raw",
			Content: "",
		}
	}

	// Default to raw for other types
	return &RequestBody{Type: "raw", Content: ""}
}

func (s *OpenAPIService) generateExampleJSON(schemaRef *openapi3.SchemaRef) string {
	if schemaRef == nil || schemaRef.Value == nil {
		return "{}"
	}

	example := s.generateSchemaExample(schemaRef.Value)
	data, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (s *OpenAPIService) generateSchemaExample(schema *openapi3.Schema) any {
	if schema.Example != nil {
		return schema.Example
	}

	if schema.Default != nil {
		return schema.Default
	}

	// Handle nil or empty type
	types := schema.Type
	if types == nil || len(types.Slice()) == 0 {
		// If no type is specified, try to infer from properties
		if len(schema.Properties) > 0 {
			obj := make(map[string]any)
			for propName, propRef := range schema.Properties {
				if propRef.Value != nil {
					obj[propName] = s.generateSchemaExample(propRef.Value)
				}
			}
			return obj
		}
		return nil
	}

	switch types.Slice()[0] {
	case "object":
		obj := make(map[string]any)
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				obj[propName] = s.generateSchemaExample(propRef.Value)
			}
		}
		return obj
	case "array":
		if schema.Items != nil && schema.Items.Value != nil {
			return []any{s.generateSchemaExample(schema.Items.Value)}
		}
		return []any{}
	case "string":
		if len(schema.Enum) > 0 {
			return schema.Enum[0]
		}
		return "string"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		return nil
	}
}

func (s *OpenAPIService) extractFormEntries(schemaRef *openapi3.SchemaRef) []KeyValue {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil
	}

	var entries []KeyValue
	schema := schemaRef.Value

	for propName, propRef := range schema.Properties {
		value := ""
		if propRef.Value != nil {
			if propRef.Value.Example != nil {
				value = fmt.Sprintf("%v", propRef.Value.Example)
			} else if propRef.Value.Default != nil {
				value = fmt.Sprintf("%v", propRef.Value.Default)
			}
		}

		required := false
		for _, req := range schema.Required {
			if req == propName {
				required = true
				break
			}
		}

		entries = append(entries, KeyValue{
			Key:     propName,
			Value:   value,
			Enabled: required,
		})
	}

	return entries
}

func (s *OpenAPIService) getExampleValue(schemaRef *openapi3.SchemaRef) string {
	if schemaRef == nil || schemaRef.Value == nil {
		return ""
	}

	schema := schemaRef.Value
	if schema.Example != nil {
		return fmt.Sprintf("%v", schema.Example)
	}
	if schema.Default != nil {
		return fmt.Sprintf("%v", schema.Default)
	}
	return ""
}

func (s *OpenAPIService) generateID(prefix, slug string) string {
	cleanSlug := s.slugify(slug)
	if len(cleanSlug) > 20 {
		cleanSlug = cleanSlug[:20]
	}
	return fmt.Sprintf("%s-%s-%d", prefix, cleanSlug, time.Now().UnixNano())
}

func (s *OpenAPIService) slugify(str string) string {
	// Remove special characters and convert to lowercase
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	slug := re.ReplaceAllString(str, "-")
	slug = strings.Trim(slug, "-")
	slug = strings.ToLower(slug)
	return slug
}
