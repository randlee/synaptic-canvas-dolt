package config

const (
	KeyDoltClient   = "dolt.client"
	KeyDoltHost     = "dolt.host"
	KeyDoltDatabase = "dolt.database"
	KeyDoltToken    = "dolt.token"
	KeyDoltDSN      = "dolt.dsn"
	KeyDoltDir      = "dolt.dir"
	KeyDoltTimeout  = "dolt.timeout"
)

var knownKeys = map[string]string{ //nolint:gosec // G101: string values are env var key names, not credentials.
	KeyDoltClient:   "SC_DOLT_CLIENT",
	KeyDoltHost:     "SC_DOLT_HOST",
	KeyDoltDatabase: "SC_DOLT_DATABASE",
	KeyDoltToken:    "SC_DOLT_TOKEN",
	KeyDoltDSN:      "SC_DOLT_DSN",
	KeyDoltDir:      "SC_DOLT_DIR",
	KeyDoltTimeout:  "SC_DOLT_TIMEOUT",
}

// IsKnownKey reports whether key is a supported config key.
func IsKnownKey(key string) bool {
	_, ok := knownKeys[key]
	return ok
}

// KnownKeys returns the supported config keys in stable order.
func KnownKeys() []string {
	return []string{
		KeyDoltClient,
		KeyDoltHost,
		KeyDoltDatabase,
		KeyDoltToken,
		KeyDoltDSN,
		KeyDoltDir,
		KeyDoltTimeout,
	}
}

// EnvNameForKey returns the environment variable name for key.
func EnvNameForKey(key string) string {
	return knownKeys[key]
}
