package appgen

import "strings"

func appPackageSource(actions []ActionRoute, ssr []SSRRoute) string {
	source := strings.ReplaceAll(appPackageSourceTemplate, "{{ACTION_HANDLER}}", actionHandlerSource(actions))
	source = strings.ReplaceAll(source, "{{SSR_HANDLER}}", ssrHandlerSource(ssr))
	return source
}
