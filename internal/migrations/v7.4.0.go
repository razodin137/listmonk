package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V7_4_0 adds the per-card captcha mode.
func V7_4_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	if _, err := db.Exec(`
		ALTER TABLE cards
		    ADD COLUMN IF NOT EXISTS captcha TEXT NOT NULL DEFAULT 'inherit';
	`); err != nil {
		return err
	}

	return nil
}
