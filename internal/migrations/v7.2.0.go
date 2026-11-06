package migrations

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V7_2_0 performs the DB migrations.
func V7_2_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS card_joins (
		    id              BIGSERIAL PRIMARY KEY,
		    card_id         INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE ON UPDATE CASCADE,
		    subscriber_id   INTEGER NULL REFERENCES subscribers(id) ON DELETE SET NULL ON UPDATE CASCADE,
		    is_new          BOOLEAN NOT NULL DEFAULT false,
		    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_card_joins_card_id ON card_joins(card_id);
		CREATE INDEX IF NOT EXISTS idx_card_joins_subscriber_id ON card_joins(subscriber_id);
		CREATE INDEX IF NOT EXISTS idx_card_joins_created_at ON card_joins(created_at);
	`); err != nil {
		return fmt.Errorf("creating card joins table: %w", err)
	}

	return nil
}
