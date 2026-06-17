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
		linked[index].ResultFields = append([]source.BackendInputField(nil), contract.ResultFields...)
		if diagnostic, bad := lookupContractDiagnostic(invalid[ref.Kind], ref); bad {
			linked[index].Status = gwdkir.ContractBindingInvalid
			linked[index].Message = diagnostic.Message
			continue
		}
		linked[index].Status = gwdkir.ContractBindingBound
	}
	return linked
}

// LinkRealtimeSubscriptions resolves GOWDK IR realtime subscriptions against
// scanned Go presentation-event registrations.
func LinkRealtimeSubscriptions(subscriptions []gwdkir.RealtimeSubscription, report Report) []gwdkir.RealtimeSubscription {
	if len(subscriptions) == 0 {
		return nil
	}
	events := map[string][]Contract{}
	for _, contract := range report.Contracts {
		if contract.Kind != runtimecontracts.Event {
			continue
		}
		for _, key := range contractReferenceKeys(contract) {
			events[key] = append(events[key], contract)
		}
	}
	invalid := map[string]Diagnostic{}
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Kind != runtimecontracts.Event {
			continue
		}
		for _, key := range diagnosticReferenceKeys(diagnostic) {
			invalid[key] = diagnostic
		}
	}
	linked := make([]gwdkir.RealtimeSubscription, len(subscriptions))
	for index, subscription := range subscriptions {
		linked[index] = subscription
		contract, ok := lookupRealtimeSubscription(events, subscription)
		if !ok {
			linked[index].Status = gwdkir.ContractBindingMissing
			linked[index].Message = fmt.Sprintf("presentation event %s has no scanned Go registration", subscription.Event)
			continue
		}
		linked[index].Handler = contract.Handler
		linked[index].Register = contract.Register
		linked[index].EventCategory = string(contract.EventCategory)
		if linked[index].EventImportPath == "" {
			linked[index].EventImportPath = contract.TypeImportPath
		}
		if linked[index].EventType == "" {
			linked[index].EventType = localContractName(contract.Type)
		}
		linked[index].Roles = append([]string(nil), contract.Roles...)
		if diagnostic, bad := lookupRealtimeSubscriptionDiagnostic(invalid, subscription); bad {
			linked[index].Status = gwdkir.ContractBindingInvalid
			linked[index].Message = diagnostic.Message
			continue
		}
		if contract.EventCategory != runtimecontracts.PresentationEvent {
			linked[index].Status = gwdkir.ContractBindingInvalid
			linked[index].Message = fmt.Sprintf("g:subscribe %s targets a %s event; subscriptions require presentation events", subscription.Event, contract.EventCategory)
			continue
		}
		linked[index].Status = gwdkir.ContractBindingBound
	}
	return linked
}

// LinkQueryInvalidations joins query references from GOWDK IR with scanned
// event-to-query invalidation registrations.
func LinkQueryInvalidations(refs []gwdkir.ContractReference, report Report) []gwdkir.QueryInvalidation {
	if len(refs) == 0 || len(report.Invalidations) == 0 {
		return nil
	}
	byQuery := map[string][]Invalidation{}
	for _, invalidation := range report.Invalidations {
		for _, key := range invalidationQueryKeys(invalidation) {
			byQuery[key] = append(byQuery[key], invalidation)
		}
	}
	var linked []gwdkir.QueryInvalidation
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.Kind != gwdkir.ContractQuery || ref.Status != gwdkir.ContractBindingBound {
			continue
		}
		for _, key := range contractReferenceLookupKeys(ref) {
			for _, invalidation := range byQuery[key] {
				eventType := runtimeTypeName(invalidation.EventTypeImportPath, invalidation.EventType)
				queryType := runtimeTypeName(ref.ImportPath, ref.Type)
				if queryType == "" {
					queryType = runtimeTypeName(invalidation.QueryTypeImportPath, invalidation.QueryType)
				}
				identity := ref.Name + "\x00" + eventType + "\x00" + ref.OwnerID + "\x00" + ref.Source
				if seen[identity] {
					continue
				}
				seen[identity] = true
				linked = append(linked, gwdkir.QueryInvalidation{
					Query:            ref.Name,
					QueryImportAlias: ref.ImportAlias,
					QueryImportPath:  ref.ImportPath,
					QueryType:        queryType,
					Event:            invalidation.EventType,
					EventImportPath:  invalidation.EventTypeImportPath,
					EventType:        eventType,
					EventCategory:    string(invalidation.EventCategory),
					Guards:           append([]string(nil), ref.Guards...),
					Status:           gwdkir.ContractBindingBound,
					OwnerKind:        ref.OwnerKind,
					OwnerID:          ref.OwnerID,
					Package:          ref.Package,
					Source:           ref.Source,
					Span:             ref.Span,
				})
			}
		}
	}
	return linked
}

func runtimeTypeName(importPath string, typ string) string {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return ""
	}
	if importPath == "" {
		return typ
	}
	return strings.TrimSpace(importPath) + "." + localContractName(typ)
}

func lookupContractReference(contracts map[string]Contract, ref gwdkir.ContractReference) (Contract, bool) {
	for _, key := range contractReferenceLookupKeys(ref) {
		if contract, ok := contracts[key]; ok {
			return contract, true
		}
	}
	return Contract{}, false
}

func lookupRealtimeSubscription(events map[string][]Contract, subscription gwdkir.RealtimeSubscription) (Contract, bool) {
	var fallback Contract
	for _, key := range realtimeSubscriptionLookupKeys(subscription) {
		for _, contract := range events[key] {
			if contract.EventCategory == runtimecontracts.PresentationEvent {
				return contract, true
			}
			if fallback.Kind == "" {
				fallback = contract
			}
		}
	}
	if fallback.Kind != "" {
		return fallback, true
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

func lookupRealtimeSubscriptionDiagnostic(diagnostics map[string]Diagnostic, subscription gwdkir.RealtimeSubscription) (Diagnostic, bool) {
	for _, key := range realtimeSubscriptionLookupKeys(subscription) {
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

func realtimeSubscriptionLookupKeys(subscription gwdkir.RealtimeSubscription) []string {
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
	add(subscription.Event)
	if subscription.EventType != "" {
		if subscription.EventImportPath != "" {
			add(contractImportTypeKey(subscription.EventImportPath, subscription.EventType))
		}
		add(subscription.EventType)
		if subscription.EventImportAlias != "" {
			add(subscription.EventImportAlias + "." + subscription.EventType)
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
