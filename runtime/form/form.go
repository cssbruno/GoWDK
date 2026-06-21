package form

import (
	"fmt"
	"mime"
	"mime/multipart"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// DefaultMultipartMemoryBytes is the generated multipart parser memory budget.
// Larger request bodies stay bounded by the generated action body limit and are
// spilled to temporary files by net/http's multipart parser.
const DefaultMultipartMemoryBytes int64 = 1 << 20

// Values is the normalized representation passed to generated action decoders.
type Values map[string][]string

// Files is the normalized representation of submitted multipart files.
type Files map[string][]File

// Data is the normalized representation passed to generated multipart action
// decoders. Values contains ordinary form fields; Files contains uploaded file
// parts.
type Data struct {
	Values Values
	Files  Files
}

// File describes one uploaded multipart file. Open returns a reader for the
// uploaded content; callers own closing it.
type File struct {
	Filename    string
	Size        int64
	ContentType string
	Header      map[string][]string

	header *multipart.FileHeader
}

// Open opens the uploaded file content for streaming by the user handler.
func (file File) Open() (multipart.File, error) {
	if file.header == nil {
		return nil, DecodeError{Message: "uploaded file is unavailable"}
	}
	return file.header.Open()
}

// FilePolicy describes the server-side limits for one expected file field.
type FilePolicy struct {
	MaxFiles            int
	MaxBytes            int64
	AllowedContentTypes []string
}

// Field describes one expected form field for generated decoders.
type Field struct {
	Name string
	File *FilePolicy
}

// Schema describes the submitted fields accepted by a generated decoder.
type Schema struct {
	Fields []Field
}

// DecodeError describes a generated form decoding failure without exposing
// submitted values.
type DecodeError struct {
	Field   string
	Message string
}

func (err DecodeError) Error() string {
	if strings.TrimSpace(err.Field) == "" {
		return err.Message
	}
	return fmt.Sprintf("%s: %s", err.Field, err.Message)
}

// FromURLValues copies request form values into a stable runtime structure.
func FromURLValues(values url.Values) Values {
	out := Values{}
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}

// FromMultipartForm copies parsed multipart values and file headers into a
// stable runtime structure.
func FromMultipartForm(multipartForm *multipart.Form) Data {
	if multipartForm == nil {
		return Data{Values: Values{}, Files: Files{}}
	}
	files := Files{}
	for key, list := range multipartForm.File {
		for _, header := range list {
			file := fileFromMultipartHeader(header)
			if file.Filename == "" && file.Size == 0 {
				continue
			}
			files[key] = append(files[key], file)
		}
	}
	return Data{
		Values: FromURLValues(url.Values(multipartForm.Value)),
		Files:  files,
	}
}

func fileFromMultipartHeader(header *multipart.FileHeader) File {
	if header == nil {
		return File{}
	}
	return File{
		Filename:    header.Filename,
		Size:        header.Size,
		ContentType: header.Header.Get("Content-Type"),
		Header:      copyHeader(header.Header),
		header:      header,
	}
}

func copyHeader(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]string, len(values))
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}

// DecodeExpected returns a copy of the submitted values restricted to the
// schema field allowlist. Missing expected fields are allowed; validation
// decides whether an absent value is acceptable.
func DecodeExpected(values Values, schema Schema) (Values, error) {
	allowed, _, err := expectedFieldPolicy(schema)
	if err != nil {
		return nil, err
	}

	for name := range values {
		if IsRuntimeField(name) {
			continue
		}
		if !allowed[name] {
			return nil, DecodeError{Field: name, Message: "unexpected field"}
		}
	}

	out := Values{}
	for _, field := range schema.Fields {
		if submitted, ok := values[field.Name]; ok {
			out[field.Name] = append([]string(nil), submitted...)
		}
	}
	return out, nil
}

// DecodeExpectedData returns a copy of submitted multipart data restricted to
// the schema field allowlist and declared file policies.
func DecodeExpectedData(data Data, schema Schema) (Data, error) {
	allowed, filePolicies, err := expectedFieldPolicy(schema)
	if err != nil {
		return Data{}, err
	}

	for name := range data.Values {
		if IsRuntimeField(name) {
			continue
		}
		if !allowed[name] {
			return Data{}, DecodeError{Field: name, Message: "unexpected field"}
		}
	}
	for name := range data.Files {
		if IsRuntimeField(name) {
			continue
		}
		if !allowed[name] || filePolicies[name] == nil {
			return Data{}, DecodeError{Field: name, Message: "unexpected file field"}
		}
	}

	out := Values{}
	for _, field := range schema.Fields {
		if submitted, ok := data.Values[field.Name]; ok {
			out[field.Name] = append([]string(nil), submitted...)
		}
	}
	files := Files{}
	for name, policy := range filePolicies {
		submitted := data.Files[name]
		checked, err := decodeExpectedFiles(name, submitted, policy)
		if err != nil {
			return Data{}, err
		}
		if len(checked) > 0 {
			files[name] = checked
		}
	}
	return Data{Values: out, Files: files}, nil
}

func expectedFieldPolicy(schema Schema) (map[string]bool, map[string]*FilePolicy, error) {
	allowed := map[string]bool{}
	filePolicies := map[string]*FilePolicy{}
	for _, field := range schema.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return nil, nil, DecodeError{Message: "expected field name is required"}
		}
		if allowed[name] {
			return nil, nil, DecodeError{Field: name, Message: "duplicate expected field"}
		}
		allowed[name] = true
		if field.File != nil {
			filePolicies[name] = field.File
		}
	}
	return allowed, filePolicies, nil
}

func decodeExpectedFiles(name string, submitted []File, policy *FilePolicy) ([]File, error) {
	if policy == nil {
		return nil, nil
	}
	if policy.MaxFiles <= 0 {
		return nil, DecodeError{Field: name, Message: "file max count is required"}
	}
	if policy.MaxBytes <= 0 {
		return nil, DecodeError{Field: name, Message: "file max size is required"}
	}
	if len(policy.AllowedContentTypes) == 0 {
		return nil, DecodeError{Field: name, Message: "file content types are required"}
	}
	if len(submitted) > policy.MaxFiles {
		return nil, DecodeError{Field: name, Message: "too many files"}
	}
	out := make([]File, 0, len(submitted))
	for _, file := range submitted {
		if file.Filename == "" && file.Size == 0 {
			continue
		}
		if file.Size > policy.MaxBytes {
			return nil, DecodeError{Field: name, Message: "file too large"}
		}
		if !contentTypeAllowed(file.ContentType, policy.AllowedContentTypes) {
			return nil, DecodeError{Field: name, Message: "file content type is not allowed"}
		}
		out = append(out, file)
	}
	return out, nil
}

func contentTypeAllowed(contentType string, allowed []string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.ToLower(strings.TrimSpace(contentType)))
	if err != nil || mediaType == "" {
		return false
	}
	for _, candidate := range allowed {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate == "" {
			continue
		}
		if strings.HasSuffix(candidate, "/*") {
			prefix := strings.TrimSuffix(candidate, "*")
			if strings.HasPrefix(mediaType, prefix) {
				return true
			}
			continue
		}
		allowedType, _, err := mime.ParseMediaType(candidate)
		if err == nil && mediaType == allowedType {
			return true
		}
	}
	return false
}

// IsRuntimeField reports whether a field is reserved for generated runtime
// metadata instead of user form input.
func IsRuntimeField(name string) bool {
	name = strings.TrimSpace(name)
	switch name {
	case "_csrf", "_gwdk", "_gowdk", "_method", "_gowdk_csrf":
		return true
	default:
		return strings.HasPrefix(name, "_gowdk_") || strings.HasPrefix(name, "_gwdk_")
	}
}

// First returns the first submitted value for a field.
func (values Values) First(name string) string {
	if len(values[name]) == 0 {
		return ""
	}
	return values[name][0]
}

// All returns all submitted values for a field.
func (values Values) All(name string) []string {
	return append([]string(nil), values[name]...)
}

// String decodes one scalar string field and rejects repeated scalar values.
func String(values Values, name string) (string, bool, error) {
	return scalar(values, name)
}

// Strings returns all submitted values for a repeated string field.
func Strings(values Values, name string) []string {
	return values.All(name)
}

// Select decodes a single-select field.
func Select(values Values, name string) (string, bool, error) {
	return String(values, name)
}

// SelectMultiple returns all submitted values for a multiple select field.
func SelectMultiple(values Values, name string) []string {
	return values.All(name)
}

// Radio decodes one selected radio value.
func Radio(values Values, name string) (string, bool, error) {
	return String(values, name)
}

// Checkbox decodes one checkbox as checked when the field was submitted.
// Absent checkboxes are false. Repeated values are rejected so checkbox groups
// use CheckboxGroup instead.
func Checkbox(values Values, name string) (bool, error) {
	submitted, ok := values[name]
	if !ok || len(submitted) == 0 {
		return false, nil
	}
	if len(submitted) > 1 {
		return false, DecodeError{Field: name, Message: "repeated checkbox field"}
	}
	switch strings.ToLower(strings.TrimSpace(submitted[0])) {
	case "", "0", "false", "off", "no":
		return false, nil
	default:
		return true, nil
	}
}

// CheckboxGroup returns all submitted values for a checkbox group.
func CheckboxGroup(values Values, name string) []string {
	return values.All(name)
}

// Bool decodes one scalar boolean field and rejects repeated scalar values.
func Bool(values Values, name string) (bool, bool, error) {
	value, ok, err := scalar(values, name)
	if err != nil || !ok {
		return false, ok, err
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "off", "no":
		return false, true, nil
	case "1", "true", "on", "yes":
		return true, true, nil
	default:
		return false, true, DecodeError{Field: name, Message: "invalid boolean"}
	}
}

// Int decodes one signed integer field with the requested bit size.
func Int(values Values, name string, bitSize int) (int64, bool, error) {
	value, ok, err := scalar(values, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true, nil
	}
	parsed, err := strconv.ParseInt(value, 10, bitSize)
	if err != nil {
		return 0, true, DecodeError{Field: name, Message: "invalid signed integer"}
	}
	return parsed, true, nil
}

// Uint decodes one unsigned integer field with the requested bit size.
func Uint(values Values, name string, bitSize int) (uint64, bool, error) {
	value, ok, err := scalar(values, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true, nil
	}
	parsed, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, true, DecodeError{Field: name, Message: "invalid unsigned integer"}
	}
	return parsed, true, nil
}

func scalar(values Values, name string) (string, bool, error) {
	submitted, ok := values[name]
	if !ok || len(submitted) == 0 {
		return "", false, nil
	}
	if len(submitted) > 1 {
		return "", true, DecodeError{Field: name, Message: "repeated scalar field"}
	}
	return submitted[0], true, nil
}

// HasSubmitted reports whether a field was submitted with at least one
// non-blank value.
func (values Values) HasSubmitted(name string) bool {
	for _, value := range values[name] {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// Names returns submitted field names in stable order.
func (values Values) Names() []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// File returns one uploaded file for a scalar file field.
func (data Data) File(name string) (File, bool, error) {
	submitted := data.Files[name]
	if len(submitted) == 0 {
		return File{}, false, nil
	}
	if len(submitted) > 1 {
		return File{}, true, DecodeError{Field: name, Message: "repeated file field"}
	}
	return submitted[0], true, nil
}

// FileList returns all uploaded files for a repeated file field.
func (data Data) FileList(name string) []File {
	return append([]File(nil), data.Files[name]...)
}

// HasSubmitted reports whether a value or file field was submitted with
// non-blank content.
func (data Data) HasSubmitted(name string) bool {
	if data.Values.HasSubmitted(name) {
		return true
	}
	for _, file := range data.Files[name] {
		if strings.TrimSpace(file.Filename) != "" || file.Size > 0 {
			return true
		}
	}
	return false
}
