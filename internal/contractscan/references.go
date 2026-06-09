package contractscan

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

func LinkReferences(refs []gwdkir.ContractReference, report Report) []gwdkir.ContractReference {
	if len(refs) == 0 {
		return nil
	}
	contracts := map[gwdkir.ContractKind]map[string]Contract{
		gwdkir.ContractCommand: {},
		gwdkir.ContractQuery:   {},
	}
	for _, contract := range report.Contracts {
		refKind, ok := irContractKind(contract.Kind)
		if !ok {
			continue
		}
		for _, key := range contractReferenceKeys(contract) {
			contracts[refKind][key] = contract
		}
	}
	invalid := map[gwdkir.ContractKind]map[string]Diagnostic{
		gwdkir.ContractCommand: {},
		gwdkir.ContractQuery:   {},
	}
	for _, diagnostic := range report.Diagnostics {
		refKind, ok := irContractKind(diagnostic.Kind)
		if !ok {
			continue
		}
		for _, key := range diagnosticReferenceKeys(diagnostic) {
			invalid[refKind][key] = diagnostic
		}
	}
	linked := make([]gwdkir.ContractReference, len(refs))
	for index, ref := range refs {
		linked[index] = ref
		kindContracts, ok := contracts[ref.Kind]
		if !ok {
			if linked[index].Status == "" {
				linked[index].Status = gwdkir.ContractBindingUnknown
			}
			continue
		}
		contract, ok := lookupContractReference(kindContracts, ref)
		if !ok {
			linked[index].Status = gwdkir.ContractBindingMissing
			linked[index].Message = fmt.Sprintf("%s %s has no scanned Go registration", ref.Kind, ref.Name)
			continue
		}
		linked[index].Handler = contract.Handler
		linked[index].Register = contract.Register
		if linked[index].Type == "" {
			linked[index].Type = contract.Type
		}
		linked[index].Result = contract.Result
		linked[index].Roles = append([]string(nil), contract.Roles...)
		linked[index].InputFields = append([]source.BackendInputField(nil), contract.InputFields...)
		if diagnostic, bad := lookupContractDiagnostic(invalid[ref.Kind], ref); bad {
			linked[index].Status = gwdkir.ContractBindingInvalid
			linked[index].Message = diagnostic.Message
			continue
		}
		linked[index].Status = gwdkir.ContractBindingBound
	}
	return linked
}

func lookupContractReference(contracts map[string]Contract, ref gwdkir.ContractReference) (Contract, bool) {
	for _, key := range contractReferenceLookupKeys(ref) {
		if contract, ok := contracts[key]; ok {
			return contract, true
		}
	}
	return Contract{}, false
}

func lookupContractDiagnostic(diagnostics map[string]Diagnostic, ref gwdkir.ContractReference) (Diagnostic, bool) {
	for _, key := range contractReferenceLookupKeys(ref) {
		if diagnostic, ok := diagnostics[key]; ok {
			return diagnostic, true
		}
	}
	return Diagnostic{}, false
}

func contractReferenceLookupKeys(ref gwdkir.ContractReference) []string {
	var keys []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, key := range keys {
			if key == value {
				return
			}
		}
		keys = append(keys, value)
	}
	add(ref.Name)
	if ref.Type != "" {
		if ref.ImportPath != "" {
			add(contractImportTypeKey(ref.ImportPath, ref.Type))
		}
		add(ref.Type)
		if ref.ImportAlias != "" {
			add(ref.ImportAlias + "." + ref.Type)
		}
	}
	return keys
}

func irContractKind(kind runtimecontracts.Kind) (gwdkir.ContractKind, bool) {
	switch kind {
	case runtimecontracts.Command:
		return gwdkir.ContractCommand, true
	case runtimecontracts.Query:
		return gwdkir.ContractQuery, true
	default:
		return "", false
	}
}

func contractReferenceKeys(contract Contract) []string {
	keys := []string{contract.Type}
	if contract.TypeImportPath != "" {
		keys = append(keys, contractImportTypeKey(contract.TypeImportPath, localContractName(contract.Type)))
	}
	if contract.Package != "" && contract.Type != "" && !strings.Contains(contract.Type, ".") {
		keys = append(keys, contract.Package+"."+contract.Type)
	}
	return keys
}

func diagnosticReferenceKeys(diagnostic Diagnostic) []string {
	keys := []string{diagnostic.Type}
	if diagnostic.TypeImportPath != "" {
		keys = append(keys, contractImportTypeKey(diagnostic.TypeImportPath, localContractName(diagnostic.Type)))
	}
	if diagnostic.Package != "" && diagnostic.Type != "" && !strings.Contains(diagnostic.Type, ".") {
		keys = append(keys, diagnostic.Package+"."+diagnostic.Type)
	}
	return keys
}

func contractImportTypeKey(importPath string, typeName string) string {
	return strings.TrimSpace(importPath) + "\x00" + strings.TrimSpace(localContractName(typeName))
}
