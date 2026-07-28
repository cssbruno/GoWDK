package gowdkcmd

import (
	"fmt"
	"strings"
)

func consumeValueFlag(args []string, index int, name string, allowEmptyEquals bool) (value string, next int, ok bool, missing bool) {
	return consumeValueFlagWithPolicy(args, index, name, valueFlagPolicy{AllowEmptyEquals: allowEmptyEquals})
}

type valueFlagPolicy struct {
	AllowEmptyEquals bool
	AllowDashValue   bool
}

func consumeValueFlagWithPolicy(args []string, index int, name string, policy valueFlagPolicy) (value string, next int, ok bool, missing bool) {
	arg := args[index]
	if arg == name {
		if index+1 >= len(args) || (!policy.AllowDashValue && flagLikeValue(args[index+1])) {
			return "", index, true, true
		}
		return args[index+1], index + 1, true, false
	}
	prefix := name + "="
	if strings.HasPrefix(arg, prefix) {
		value := strings.TrimPrefix(arg, prefix)
		if (!policy.AllowEmptyEquals && value == "") || (!policy.AllowDashValue && flagLikeValue(value)) {
			return "", index, true, true
		}
		return value, index, true, false
	}
	return "", index, false, false
}

func flagLikeValue(value string) bool {
	return strings.HasPrefix(value, "-")
}

func missingValueFlagError(name string) error {
	return fmt.Errorf("%s requires a value", name)
}
