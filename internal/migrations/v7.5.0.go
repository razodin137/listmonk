package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V7_5_0 adds the post campaign type for posts published only to the web.
func V7_5_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	if _, err := db.Exec(`ALTER TYPE campaign_type ADD VALUE IF NOT EXISTS 'post'`); err != nil {
		return err
	}

	return nil
}
