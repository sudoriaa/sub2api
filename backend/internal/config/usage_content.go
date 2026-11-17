package config

// UsageContentConfig controls the per-process, admin-only memory window.
// The 24-hour TTL is an upper bound, not a durability guarantee.
type UsageContentConfig struct {
	Enabled      bool  `mapstructure:"enabled"`
	MaxBodyBytes int   `mapstructure:"max_body_bytes"`
	MaxRecords   int   `mapstructure:"max_records"`
	MaxMemoryMiB int64 `mapstructure:"max_memory_mib"`
}
