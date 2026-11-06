package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V7_3_0 adds the per-card success message and redirect shown after a
// successful signup.
func V7_3_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	if _, err := db.Exec(`
		ALTER TABLE cards
		    ADD COLUMN IF NOT EXISTS success_message TEXT NOT NULL DEFAULT '',
		    ADD COLUMN IF NOT EXISTS redirect_url TEXT NOT NULL DEFAULT '';
	`); err != nil {
		return err
	}

	return nil
}
