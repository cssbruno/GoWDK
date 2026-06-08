package parser

import "regexp"

var (
	annotationPattern       = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_]*)\s*(.*)$`)
	packagePattern          = regexp.MustCompile(`^package\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	importPattern           = regexp.MustCompile(`^import(?:\s+([A-Za-z_][A-Za-z0-9_]*))?\s+"([^"]+)"$`)
	usePattern              = regexp.MustCompile(`^use\s+([A-Za-z_][A-Za-z0-9_]*)\s+"([A-Za-z_][A-Za-z0-9_]*)"$`)
	jsPattern               = regexp.MustCompile(`^js\s+"([^"]+)"$`)
	buildCallPattern        = regexp.MustCompile(`^=>\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\)$`)
	actionEndpointPattern   = regexp.MustCompile(`^act\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Z]+)\s+"([^"]*)"(?:\s+@error\s+"([^"]*)")?$`)
	apiEndpointPattern      = regexp.MustCompile(`^api\s+([A-Za-z_][A-Za-z0-9_]*)\s+(GET|POST|PUT|PATCH|DELETE)\s+"([^"]*)"(?:\s+@error\s+"([^"]*)")?$`)
	fragmentEndpointPattern = regexp.MustCompile(`^fragment\s+([A-Za-z_][A-Za-z0-9_]*)\s+(GET|POST|PUT|PATCH|DELETE)\s+"([^"]*)"\s+"([^"]*)"\s*\{$`)
	actionPattern           = regexp.MustCompile(`^act\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*\{`)
	apiPattern              = regexp.MustCompile(`^api(?:\s+([A-Za-z_][A-Za-z0-9_.-]*))?\s*\{`)
	propPattern             = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	emitPattern             = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*\((.*)\)$`)
	identifierPattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	componentTypePattern    = regexp.MustCompile(`^(props|state)\s+([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)(?:\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\))?$`)
	storePattern            = regexp.MustCompile(`^store\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\)$`)
	actionInputPattern      = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*form\s+([A-Za-z_][A-Za-z0-9_]*)$`)
	actionValidPattern      = regexp.MustCompile(`^valid\(([A-Za-z_][A-Za-z0-9_]*)\)\?$`)
	actionRedirectPattern   = regexp.MustCompile(`^->\s*"([^"]*)"$`)
	actionFragmentPattern   = regexp.MustCompile(`^fragment\s+"([^"]*)"\s*\{$`)
	apiRoutePattern         = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE)\s+"([^"]*)"$`)
	routeParamPattern       = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(?::([A-Za-z_][A-Za-z0-9_]*))?\}`)
)
