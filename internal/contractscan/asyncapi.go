package contractscan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/source"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

const AsyncAPIFile = "asyncapi.json"

type AsyncAPIOptions struct {
	IncludeDomainEvents bool
}

type asyncAPISpec struct {
	AsyncAPI   string                       `json:"asyncapi"`
	Info       asyncAPIInfo                 `json:"info"`
	Channels   map[string]asyncAPIChannel   `json:"channels"`
	Operations map[string]asyncAPIOperation `json:"operations"`
	Components asyncAPIComponents           `json:"components,omitempty"`
	XGOWDK     map[string]string            `json:"x-gowdk,omitempty"`
}

type asyncAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type asyncAPIChannel struct {
	Address  string                     `json:"address"`
	Messages map[string]asyncAPIMessage `json:"messages,omitempty"`
	XGOWDK   map[string]string          `json:"x-gowdk,omitempty"`
}

type asyncAPIMessage struct {
	Name    string            `json:"name"`
	Title   string            `json:"title,omitempty"`
	Payload asyncAPISchema    `json:"payload"`
	Traits  []asyncAPIRef     `json:"traits,omitempty"`
	XGOWDK  map[string]string `json:"x-gowdk,omitempty"`
}

type asyncAPIOperation struct {
	Action   string            `json:"action"`
	Channel  asyncAPIRef       `json:"channel"`
	Messages []asyncAPIRef     `json:"messages,omitempty"`
	XGOWDK   map[string]string `json:"x-gowdk,omitempty"`
}

type asyncAPIComponents struct {
	Schemas       map[string]asyncAPISchema       `json:"schemas,omitempty"`
	MessageTraits map[string]asyncAPIMessageTrait `json:"messageTraits,omitempty"`
}

type asyncAPIMessageTrait struct {
	Headers asyncAPISchema `json:"headers,omitempty"`
}

type asyncAPIRef struct {
	Ref string `json:"$ref"`
}

type asyncAPISchema struct {
	Ref                  string                    `json:"$ref,omitempty"`
	Type                 string                    `json:"type,omitempty"`
	Format               string                    `json:"format,omitempty"`
	Items                *asyncAPISchema           `json:"items,omitempty"`
	Properties           map[string]asyncAPISchema `json:"properties,omitempty"`
	AdditionalProperties *bool                     `json:"additionalProperties,omitempty"`
	XGoType              string                    `json:"x-go-type,omitempty"`
}

func WriteAsyncAPI(outputDir string, report Report, options AsyncAPIOptions) (string, error) {
	payload, err := AsyncAPIPayload(report, options)
	if err != nil {
		return "", err
	}
	path := filepath.Join(outputDir, AsyncAPIFile)
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, payload) {
		return path, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func AsyncAPIPayload(report Report, options AsyncAPIOptions) ([]byte, error) {
	spec := buildAsyncAPISpec(report, options)
	payload, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func buildAsyncAPISpec(report Report, options AsyncAPIOptions) asyncAPISpec {
	events := asyncAPIEvents(report, options)
	channels := map[string]asyncAPIChannel{}
	operations := map[string]asyncAPIOperation{}
	schemas := map[string]asyncAPISchema{}
	for _, event := range events {
		category := string(event.EventCategory)
		eventName := localContractName(event.Type)
		if eventName == "" {
			eventName = schemaName(event.Type)
		}
		channelName := category + "." + eventName
		schemaName := schemaName(event.Type)
		schemas[schemaName] = asyncAPISchemaForContract(event)
		message := asyncAPIMessage{
			Name:    eventName,
			Title:   event.Type,
			Payload: asyncAPISchema{Ref: "#/components/schemas/" + schemaName},
			Traits:  []asyncAPIRef{{Ref: "#/components/messageTraits/cloudEvents"}},
			XGOWDK: map[string]string{
				"kind":     string(event.Kind),
				"category": category,
				"source":   event.Source,
				"handler":  event.Handler,
			},
		}
		channels[channelName] = asyncAPIChannel{
			Address: channelName,
			Messages: map[string]asyncAPIMessage{
				eventName: message,
			},
			XGOWDK: map[string]string{
				"category": category,
				"goType":   event.Type,
			},
		}
		operationName := "publish" + exportedIdentifier(category) + exportedIdentifier(eventName)
		operations[operationName] = asyncAPIOperation{
			Action:  "send",
			Channel: asyncAPIRef{Ref: "#/channels/" + channelName},
			Messages: []asyncAPIRef{
				{Ref: "#/channels/" + channelName + "/messages/" + eventName},
			},
			XGOWDK: map[string]string{
				"category": category,
				"goType":   event.Type,
			},
		}
	}
	return asyncAPISpec{
		AsyncAPI:   "3.0.0",
		Info:       asyncAPIInfo{Title: "GOWDK contract events", Version: "0"},
		Channels:   channels,
		Operations: operations,
		Components: asyncAPIComponents{
			Schemas:       schemas,
			MessageTraits: cloudEventsMessageTraits(),
		},
		XGOWDK: map[string]string{"schema": "gowdk.asyncapi.v1"},
	}
}

func asyncAPIEvents(report Report, options AsyncAPIOptions) []Contract {
	var events []Contract
	for _, contract := range report.Contracts {
		if contract.Kind != runtimecontracts.Event {
			continue
		}
		switch contract.EventCategory {
		case runtimecontracts.IntegrationEvent:
			events = append(events, contract)
		case runtimecontracts.DomainEvent:
			if options.IncludeDomainEvents {
				events = append(events, contract)
			}
		}
	}
	sort.Slice(events, func(i, j int) bool {
		left := string(events[i].EventCategory) + "\x00" + events[i].Type + "\x00" + events[i].Source
		right := string(events[j].EventCategory) + "\x00" + events[j].Type + "\x00" + events[j].Source
		return left < right
	})
	return events
}

func asyncAPISchemaForContract(contract Contract) asyncAPISchema {
	schema := asyncAPIObjectSchema(contract.PayloadFields)
	schema.XGoType = contract.Type
	if len(schema.Properties) == 0 {
		return asyncAPISchema{Type: "object", XGoType: contract.Type}
	}
	return schema
}

func asyncAPIObjectSchema(fields []source.BackendInputField) asyncAPISchema {
	properties := map[string]asyncAPISchema{}
	for _, field := range fields {
		name := field.FormName
		if name == "" {
			name = field.FieldName
		}
		if name == "" {
			continue
		}
		properties[name] = asyncAPISchemaForGoType(field.Type)
	}
	return asyncAPISchema{Type: "object", Properties: properties}
}

func asyncAPISchemaForGoType(goType string) asyncAPISchema {
	fieldType := source.MustBackendInputFieldType(goType)
	switch fieldType.Kind {
	case source.BackendInputFieldKindBool:
		return asyncAPISchema{Type: "boolean"}
	case source.BackendInputFieldKindSignedInt, source.BackendInputFieldKindUnsignedInt:
		return asyncAPISchema{Type: "integer", Format: "int64"}
	case source.BackendInputFieldKindStringSlice:
		item := asyncAPISchema{Type: "string"}
		return asyncAPISchema{Type: "array", Items: &item}
	case source.BackendInputFieldKindFile:
		return asyncAPISchema{Type: "string", Format: "binary"}
	case source.BackendInputFieldKindFileSlice:
		item := asyncAPISchema{Type: "string", Format: "binary"}
		return asyncAPISchema{Type: "array", Items: &item}
	case source.BackendInputFieldKindString:
		return asyncAPISchema{Type: "string"}
	default:
		panic("unsupported backend input field kind: " + string(fieldType.Kind))
	}
}

func cloudEventsMessageTraits() map[string]asyncAPIMessageTrait {
	return map[string]asyncAPIMessageTrait{
		"cloudEvents": {
			Headers: asyncAPISchema{
				Type: "object",
				Properties: map[string]asyncAPISchema{
					"specversion":     {Type: "string"},
					"id":              {Type: "string"},
					"source":          {Type: "string"},
					"type":            {Type: "string"},
					"time":            {Type: "string", Format: "date-time"},
					"datacontenttype": {Type: "string"},
				},
			},
		},
	}
}

func schemaName(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndex(value, "."); index >= 0 {
		value = value[index+1:]
	}
	name := exportedIdentifier(value)
	if name == "" {
		return "GOWDKEvent"
	}
	return name
}

func exportedIdentifier(value string) string {
	var out []rune
	upperNext := true
	for _, r := range value {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			if len(out) == 0 && unicode.IsDigit(r) {
				out = append(out, '_')
			}
			if upperNext {
				out = append(out, unicode.ToUpper(r))
				upperNext = false
				continue
			}
			out = append(out, r)
			continue
		}
		upperNext = len(out) > 0
	}
	return string(out)
}
