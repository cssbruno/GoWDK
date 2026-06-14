package source

import (
	"sort"
	"strings"
)

type BackendInputFieldKind string

const (
	BackendInputFieldKindString      BackendInputFieldKind = "string"
	BackendInputFieldKindBool        BackendInputFieldKind = "bool"
	BackendInputFieldKindSignedInt   BackendInputFieldKind = "signed_int"
	BackendInputFieldKindUnsignedInt BackendInputFieldKind = "unsigned_int"
	BackendInputFieldKindStringSlice BackendInputFieldKind = "string_slice"
)

const (
	BackendInputTypeString      = "string"
	BackendInputTypeBool        = "bool"
	BackendInputTypeInt         = "int"
	BackendInputTypeInt8        = "int8"
	BackendInputTypeInt16       = "int16"
	BackendInputTypeInt32       = "int32"
	BackendInputTypeInt64       = "int64"
	BackendInputTypeUint        = "uint"
	BackendInputTypeUint8       = "uint8"
	BackendInputTypeUint16      = "uint16"
	BackendInputTypeUint32      = "uint32"
	BackendInputTypeUint64      = "uint64"
	BackendInputTypeStringSlice = "[]string"
)

// BackendInputFieldTypeInfo describes one Go type accepted for generated
// backend input decoding and contract input metadata.
type BackendInputFieldTypeInfo struct {
	Name    string
	Kind    BackendInputFieldKind
	BitSize int
}

var backendInputFieldTypes = map[string]BackendInputFieldTypeInfo{
	BackendInputTypeString:      {Name: BackendInputTypeString, Kind: BackendInputFieldKindString},
	BackendInputTypeBool:        {Name: BackendInputTypeBool, Kind: BackendInputFieldKindBool},
	BackendInputTypeInt:         {Name: BackendInputTypeInt, Kind: BackendInputFieldKindSignedInt},
	BackendInputTypeInt8:        {Name: BackendInputTypeInt8, Kind: BackendInputFieldKindSignedInt, BitSize: 8},
	BackendInputTypeInt16:       {Name: BackendInputTypeInt16, Kind: BackendInputFieldKindSignedInt, BitSize: 16},
	BackendInputTypeInt32:       {Name: BackendInputTypeInt32, Kind: BackendInputFieldKindSignedInt, BitSize: 32},
	BackendInputTypeInt64:       {Name: BackendInputTypeInt64, Kind: BackendInputFieldKindSignedInt, BitSize: 64},
	BackendInputTypeUint:        {Name: BackendInputTypeUint, Kind: BackendInputFieldKindUnsignedInt},
	BackendInputTypeUint8:       {Name: BackendInputTypeUint8, Kind: BackendInputFieldKindUnsignedInt, BitSize: 8},
	BackendInputTypeUint16:      {Name: BackendInputTypeUint16, Kind: BackendInputFieldKindUnsignedInt, BitSize: 16},
	BackendInputTypeUint32:      {Name: BackendInputTypeUint32, Kind: BackendInputFieldKindUnsignedInt, BitSize: 32},
	BackendInputTypeUint64:      {Name: BackendInputTypeUint64, Kind: BackendInputFieldKindUnsignedInt, BitSize: 64},
	BackendInputTypeStringSlice: {Name: BackendInputTypeStringSlice, Kind: BackendInputFieldKindStringSlice},
}

// LookupBackendInputFieldType returns the canonical metadata for a supported
// backend input field type.
func LookupBackendInputFieldType(name string) (BackendInputFieldTypeInfo, bool) {
	info, ok := backendInputFieldTypes[strings.TrimSpace(name)]
	return info, ok
}

// MustBackendInputFieldType returns the metadata for name and panics when a
// compiler-generated backend input field carries an unsupported type.
func MustBackendInputFieldType(name string) BackendInputFieldTypeInfo {
	info, ok := LookupBackendInputFieldType(name)
	if !ok {
		panic("unsupported backend input field type: " + name)
	}
	return info
}

func SupportedBackendInputFieldTypes() []string {
	names := make([]string, 0, len(backendInputFieldTypes))
	for name := range backendInputFieldTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
