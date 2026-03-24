// Package openapi generates an OpenAPI 3.0 spec from the schema IR.
package openapi

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/fisayoafolayan/kiln/internal/config"
	"github.com/fisayoafolayan/kiln/internal/generator/genopt"
	"github.com/fisayoafolayan/kiln/internal/ir"
)

// Generator writes a single openapi.yaml file covering all tables.
type Generator struct {
	opts genopt.Options
	tmpl *template.Template
}

// New returns a Generator ready to run.
func New(opts genopt.Options) (*Generator, error) {
	tmpl, err := template.New("openapi").Funcs(funcMap()).Parse(openapiTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing openapi template: %w", err)
	}
	return &Generator{opts: opts, tmpl: tmpl}, nil
}

// Run generates the OpenAPI spec file.
func (g *Generator) Run() ([]string, error) {
	cfg := g.opts.Config
	outPath := cfg.OpenAPI.Output
	if outPath == "" {
		outPath = "./docs/openapi.yaml"
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return nil, fmt.Errorf("creating docs dir: %w", err)
	}

	data := g.templateData()

	var buf bytes.Buffer
	if err := g.tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing openapi template: %w", err)
	}

	skipped, err := genopt.WriteRawWithChecksum(buf.Bytes(), outPath, g.opts.Force)
	if err != nil {
		return nil, err
	}
	if skipped {
		return nil, nil
	}
	return []string{outPath}, nil
}

// Diff returns the list of files that would be written without writing them.
func (g *Generator) Diff() []string {
	out := g.opts.Config.OpenAPI.Output
	if out == "" {
		out = "./docs/openapi.yaml"
	}
	return []string{out}
}

// ---------------------------------------------------------------------------
// Template data
// ---------------------------------------------------------------------------

type templateData struct {
	Title        string
	Version      string
	Description  string
	BasePath     string
	Tables       []tableData
	AuthStrategy string // "none", "jwt", "api_key"
	AuthHeader   string
}

type tableData struct {
	Table    *ir.Table
	Endpoint string
	Override config.TableOverride
	Fields   []fieldData
	// Writable fields for request body schemas
	CreateFields  []fieldData
	UpdateFields  []fieldData
	FilterFields  []fieldData // filterable columns with OpenAPI type info
	SortableNames []string    // sortable column JSON names
	// Nested routes from FK relationships
	NestedRoutes []nestedRoute
}

type nestedRoute struct {
	ParentTable    *ir.Table
	ParentEndpoint string
}

type fieldData struct {
	Name        string
	JSONName    string
	OAPIType    string // OpenAPI type: string, integer, number, boolean
	OAPIFormat  string // OpenAPI format: uuid, date-time, int64 etc.
	Nullable    bool
	Required    bool
	Description string
}

func (g *Generator) templateData() templateData {
	cfg := g.opts.Config
	data := templateData{
		Title:        cfg.OpenAPI.Title,
		Version:      cfg.OpenAPI.Version,
		Description:  cfg.OpenAPI.Description,
		BasePath:     cfg.API.BasePath,
		AuthStrategy: cfg.Auth.Strategy,
		AuthHeader:   cfg.Auth.Header,
	}

	for _, t := range g.opts.Schema.Tables {
		if !cfg.ShouldGenerateTable(t.Name) {
			continue
		}

		override := cfg.OverrideFor(t.Name)
		endpoint := t.Endpoint()
		if override.Endpoint != "" {
			endpoint = override.Endpoint
		}

		td := tableData{
			Table:    t,
			Endpoint: endpoint,
			Override: override,
		}

		for _, c := range t.Columns {
			if override.IsFieldHidden(c.Name) || c.IsSoftDeleteColumn() {
				continue
			}
			fd := toFieldData(c)
			td.Fields = append(td.Fields, fd)

			if !c.IsReadOnly() && !override.IsFieldReadonly(c.Name) {
				createFD := fd
				createFD.Required = !c.Nullable && !c.HasDefault
				td.CreateFields = append(td.CreateFields, createFD)

				updateFD := fd
				updateFD.Required = false // all fields optional on update
				td.UpdateFields = append(td.UpdateFields, updateFD)
			}
		}

		for _, c := range t.Columns {
			if override.IsFieldFilterable(c.Name) && c.GoType.IsFilterable() {
				td.FilterFields = append(td.FilterFields, toFieldData(c))
			}
			if override.IsFieldSortable(c.Name) && c.GoType.IsFilterable() {
				td.SortableNames = append(td.SortableNames, c.JSONName())
			}
		}

		for _, fk := range t.ForeignKeys {
			parentOverride := cfg.OverrideFor(fk.TargetTable.Name)
			parentEndpoint := fk.TargetTable.Endpoint()
			if parentOverride.Endpoint != "" {
				parentEndpoint = parentOverride.Endpoint
			}
			td.NestedRoutes = append(td.NestedRoutes, nestedRoute{
				ParentTable:    fk.TargetTable,
				ParentEndpoint: parentEndpoint,
			})
		}

		data.Tables = append(data.Tables, td)
	}

	return data
}

// toFieldData converts an ir.Column to a fieldData with OpenAPI type mappings.
func toFieldData(c *ir.Column) fieldData {
	oapiType, oapiFormat := goTypeToOAPI(c.GoType)
	return fieldData{
		Name:       c.GoName(),
		JSONName:   c.JSONName(),
		OAPIType:   oapiType,
		OAPIFormat: oapiFormat,
		Nullable:   c.Nullable,
		Required:   !c.Nullable && !c.HasDefault && !c.IsPrimaryKey,
	}
}

// goTypeToOAPI maps a kiln GoType to an OpenAPI type + format pair.
func goTypeToOAPI(t ir.GoType) (oapiType, oapiFormat string) {
	switch t.Name {
	case "string":
		return "string", ""
	case "int32":
		return "integer", "int32"
	case "int64":
		return "integer", "int64"
	case "float32", "float64":
		return "number", "float"
	case "bool":
		return "boolean", ""
	case "time.Time":
		return "string", "date-time"
	case "uuid.UUID":
		return "string", "uuid"
	case "json.RawMessage":
		return "object", ""
	case "[]byte":
		return "string", "byte"
	default:
		return "string", ""
	}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"isOperationEnabled": func(op string, o config.TableOverride) bool {
			return !o.IsOperationDisabled(op)
		},
		"hasFormat": func(f string) bool {
			return f != ""
		},
		"isLast": func(i, total int) bool {
			return i == total-1
		},
		"needsRangeOps": func(f fieldData) bool {
			switch f.OAPIType {
			case "integer", "number":
				return true
			}
			return f.OAPIFormat == "date-time"
		},
	}
}

// ---------------------------------------------------------------------------
// OpenAPI template
// ---------------------------------------------------------------------------

const openapiTemplate = `# Generated by kiln. DO NOT EDIT.
# Re-generated on each run. Use overrides in kiln.yaml to customise.
# kiln:checksum=__CHECKSUM__
openapi: "3.0.3"

info:
  title: {{.Title}}
  version: {{.Version}}
{{- if .Description}}
  description: {{.Description}}
{{- end}}
{{if eq .AuthStrategy "jwt"}}
security:
  - bearerAuth: []
{{end}}{{if eq .AuthStrategy "api_key"}}
security:
  - apiKeyAuth: []
{{end}}
paths:
{{range .Tables}}
{{- $table := . }}
  # ------------------------------------------------------------
  # {{$table.Table.Name}}
  # ------------------------------------------------------------

{{if isOperationEnabled "list" .Override}}  {{$.BasePath}}/{{.Endpoint}}:
    get:
      summary: List {{.Table.GoNamePlural}}
      operationId: list{{.Table.GoNamePlural}}
      tags: [{{.Table.GoName}}]
      parameters:
        - name: page
          in: query
          schema: { type: integer, default: 1 }
        - name: page_size
          in: query
          schema: { type: integer, default: 20, maximum: 100 }
{{range .FilterFields}}        - name: {{.JSONName}}
          in: query
          description: "Filter by {{.JSONName}} (exact match)"
          schema: { type: {{.OAPIType}}{{if hasFormat .OAPIFormat}}, format: {{.OAPIFormat}}{{end}} }
        - name: {{.JSONName}}[neq]
          in: query
          description: "Filter by {{.JSONName}} (not equal)"
          schema: { type: {{.OAPIType}}{{if hasFormat .OAPIFormat}}, format: {{.OAPIFormat}}{{end}} }
{{if needsRangeOps .}}        - name: {{.JSONName}}[gt]
          in: query
          schema: { type: {{.OAPIType}}{{if hasFormat .OAPIFormat}}, format: {{.OAPIFormat}}{{end}} }
        - name: {{.JSONName}}[gte]
          in: query
          schema: { type: {{.OAPIType}}{{if hasFormat .OAPIFormat}}, format: {{.OAPIFormat}}{{end}} }
        - name: {{.JSONName}}[lt]
          in: query
          schema: { type: {{.OAPIType}}{{if hasFormat .OAPIFormat}}, format: {{.OAPIFormat}}{{end}} }
        - name: {{.JSONName}}[lte]
          in: query
          schema: { type: {{.OAPIType}}{{if hasFormat .OAPIFormat}}, format: {{.OAPIFormat}}{{end}} }
{{end}}{{end}}{{if .SortableNames}}        - name: sort
          in: query
          description: "Sort by field. Prefix with - for descending."
          schema: { type: string, enum: [{{range $i, $n := .SortableNames}}{{if $i}}, {{end}}{{$n}}, -{{$n}}{{end}}] }
{{end}}      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: "#/components/schemas/{{.Table.GoName}}"
                  total: { type: integer }
                  page: { type: integer }
                  page_size: { type: integer }
        "500":
          $ref: "#/components/responses/InternalError"
{{end}}
{{if isOperationEnabled "create" .Override}}    post:
      summary: Create {{.Table.GoName}}
      operationId: create{{.Table.GoName}}
      tags: [{{.Table.GoName}}]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Create{{.Table.GoName}}Request"
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/{{.Table.GoName}}"
        "400":
          $ref: "#/components/responses/BadRequest"
        "422":
          $ref: "#/components/responses/ValidationError"
        "500":
          $ref: "#/components/responses/InternalError"
{{end}}
{{if isOperationEnabled "get" .Override}}  {{$.BasePath}}/{{.Endpoint}}/{id}:
    get:
      summary: Get {{.Table.GoName}} by ID
      operationId: get{{.Table.GoName}}
      tags: [{{.Table.GoName}}]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/{{.Table.GoName}}"
        "404":
          $ref: "#/components/responses/NotFound"
        "500":
          $ref: "#/components/responses/InternalError"
{{end}}
{{if isOperationEnabled "update" .Override}}    patch:
      summary: Update {{.Table.GoName}}
      operationId: update{{.Table.GoName}}
      tags: [{{.Table.GoName}}]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Update{{.Table.GoName}}Request"
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/{{.Table.GoName}}"
        "400":
          $ref: "#/components/responses/BadRequest"
        "404":
          $ref: "#/components/responses/NotFound"
        "422":
          $ref: "#/components/responses/ValidationError"
        "500":
          $ref: "#/components/responses/InternalError"
{{end}}
{{if isOperationEnabled "delete" .Override}}    delete:
      summary: Delete {{.Table.GoName}}
      operationId: delete{{.Table.GoName}}
      tags: [{{.Table.GoName}}]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
      responses:
        "204":
          description: No Content
        "404":
          $ref: "#/components/responses/NotFound"
        "500":
          $ref: "#/components/responses/InternalError"
{{end}}
{{range .NestedRoutes}}
  {{$.BasePath}}/{{.ParentEndpoint}}/{id}/{{$table.Endpoint}}:
    get:
      summary: List {{$table.Table.GoNamePlural}} by {{.ParentTable.GoName}}
      operationId: list{{$table.Table.GoNamePlural}}By{{.ParentTable.GoName}}
      tags: [{{$table.Table.GoName}}]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
        - name: page
          in: query
          schema: { type: integer, default: 1 }
        - name: page_size
          in: query
          schema: { type: integer, default: 20, maximum: 100 }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: array
                    items:
                      $ref: "#/components/schemas/{{$table.Table.GoName}}"
                  total: { type: integer }
                  page: { type: integer }
                  page_size: { type: integer }
        "404":
          $ref: "#/components/responses/NotFound"
        "500":
          $ref: "#/components/responses/InternalError"
{{end}}
{{end}}

components:
  schemas:
{{range .Tables}}
    # {{.Table.GoName}}
    {{.Table.GoName}}:
      type: object
      properties:
{{range .Fields}}        {{.JSONName}}:
          type: {{.OAPIType}}
{{- if hasFormat .OAPIFormat}}
          format: {{.OAPIFormat}}
{{- end}}
{{- if .Nullable}}
          nullable: true
{{- end}}
{{end}}
      required:
{{range .Fields}}{{if .Required}}        - {{.JSONName}}
{{end}}{{end}}

    Create{{.Table.GoName}}Request:
      type: object
      properties:
{{range .CreateFields}}        {{.JSONName}}:
          type: {{.OAPIType}}
{{- if hasFormat .OAPIFormat}}
          format: {{.OAPIFormat}}
{{- end}}
{{end}}
      required:
{{range .CreateFields}}{{if .Required}}        - {{.JSONName}}
{{end}}{{end}}

    Update{{.Table.GoName}}Request:
      type: object
      properties:
{{range .UpdateFields}}        {{.JSONName}}:
          type: {{.OAPIType}}
{{- if hasFormat .OAPIFormat}}
          format: {{.OAPIFormat}}
{{- end}}
{{end}}

{{end}}
    Error:
      type: object
      properties:
        error:
          type: string
      required: [error]

  responses:
    BadRequest:
      description: Bad Request
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
    NotFound:
      description: Not Found
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
    ValidationError:
      description: Unprocessable Entity
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
    InternalError:
      description: Internal Server Error
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
{{if eq .AuthStrategy "jwt"}}
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
{{end}}{{if eq .AuthStrategy "api_key"}}
  securitySchemes:
    apiKeyAuth:
      type: apiKey
      in: header
      name: {{.AuthHeader}}
{{end}}`
