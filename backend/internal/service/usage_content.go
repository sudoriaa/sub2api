package service

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
)

func ProvideUsageContentManager(cfg *config.Config) *usagecontent.Manager {
	c := usagecontent.Config{Enabled: true, MaxBodyBytes: usagecontent.DefaultMaxBodyBytes}
	if cfg != nil {
		c.Enabled = cfg.Gateway.UsageContent.Enabled
		c.MaxBodyBytes = cfg.Gateway.UsageContent.MaxBodyBytes
		c.MaxRecords = cfg.Gateway.UsageContent.MaxRecords
		c.MaxMemoryBytes = cfg.Gateway.UsageContent.MaxMemoryMiB << 20
	}
	return usagecontent.New(c)
}
