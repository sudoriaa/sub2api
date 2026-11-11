package service

import (
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/usagecontent"
)

func ProvideUsageContentManager(db *sql.DB, cfg *config.Config) *usagecontent.Manager {
	c := usagecontent.Config{Enabled: true, MaxBodyBytes: usagecontent.DefaultMaxBodyBytes}
	storageMiB := int64(usagecontent.DefaultStorageMiB)
	if cfg != nil {
		c.Enabled = cfg.Gateway.UsageContent.Enabled
		c.MaxBodyBytes = cfg.Gateway.UsageContent.MaxBodyBytes
		if cfg.Gateway.UsageContent.MaxStorageMiB > 0 {
			storageMiB = cfg.Gateway.UsageContent.MaxStorageMiB
		}
	}
	return usagecontent.New(&usagecontent.SQLStore{DB: db, MaxStorageBytes: storageMiB << 20}, c)
}
