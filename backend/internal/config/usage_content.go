package config

// UsageContentConfig controls admin-only, fixed 24-hour input/output history.
// No setting can lengthen retention; the body limit bounds diagnostic overhead.
type UsageContentConfig struct {
	Enabled       bool  `mapstructure:"enabled"`
	MaxBodyBytes  int   `mapstructure:"max_body_bytes"`
	MaxStorageMiB int64 `mapstructure:"max_storage_mib"`
}
