package gowdk

// BuildConfig controls output artifacts and frontend asset packaging.
type BuildConfig struct {
	Output              string
	Mode                BuildMode
	Assets              AssetMode
	ObfuscateAssets     bool
	Head                HeadConfig
	CSRF                CSRFConfig
	CORS                CORSConfig
	SecurityHeaders     SecurityHeadersConfig
	BodyLimits          BodyLimitsConfig
	AllowMissingBackend bool
	Stylesheets         []Stylesheet
	Scripts             []Script
	Worker              ContractWorkerConfig
	Cron                ContractCronConfig
	Targets             []BuildTargetConfig
}

type HeadConfig struct {
	SiteName    string
	Favicon     string
	Image       string
	TwitterCard string
}

type SecurityHeadersConfig struct {
	Enabled bool
	Headers map[string]string
}

type CORSConfig struct {
	Enabled          bool
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
}

// CSRFConfig is enabled by default. Disabled is its single explicit opt-out.
type CSRFConfig struct {
	Disabled               bool
	SecretEnv              string
	VerificationSecretEnvs []string
	CookieName             string
	FieldName              string
	HeaderName             string
	Insecure               bool
}

type BodyLimitsConfig struct {
	ActionBytes int64
	APIBytes    int64
}

// BuildTargetConfig declares one independently publishable artifact set.
type BuildTargetConfig struct {
	Name          string
	Modules       []string
	Output        string
	App           string
	Binary        string
	WASM          string
	BackendApp    string
	BackendBinary string
	WorkerApp     string
	WorkerBinary  string
	Worker        ContractWorkerConfig
	CronApp       string
	CronBinary    string
	Cron          ContractCronConfig
	DeployRecipes []string
}

type ContractWorkerConfig struct {
	EventSource ServiceRef
	SeenStore   ServiceRef
	Backoff     ServiceRef
}

type ContractCronConfig struct {
	Jobs []ContractCronJobConfig
}

type ContractCronJobConfig struct {
	Type            string
	Schedule        string
	OverlapPolicy   string
	MissedRunPolicy string
}

type AssetMode string

const (
	AssetExternal AssetMode = "external"
	Embed         AssetMode = "embed"
)

type BuildMode string

const (
	Development BuildMode = "development"
	Production  BuildMode = "production"
)
