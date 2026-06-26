package gowdkcmd

import "strings"

func consumeValueFlag(args []string, index int, name string, allowEmptyEquals bool) (value string, next int, ok bool, missing bool) {
	arg := args[index]
	if arg == name {
		if index+1 >= len(args) {
			return "", index, true, true
		}
		return args[index+1], index + 1, true, false
	}
	prefix := name + "="
	if strings.HasPrefix(arg, prefix) {
		if !allowEmptyEquals && len(arg) == len(prefix) {
			return "", index, false, false
		}
		return strings.TrimPrefix(arg, prefix), index, true, false
	}
	return "", index, false, false
}
