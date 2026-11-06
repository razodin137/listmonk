package migrations

import (
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/knadh/koanf/v2"
	"github.com/knadh/stuffbin"
)

// V7_1_0 performs the DB migrations.
func V7_1_0(db *sqlx.DB, fs stuffbin.FileSystem, ko *koanf.Koanf, lo *log.Logger) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cards (
		    id              SERIAL PRIMARY KEY,
		    uuid            uuid NOT NULL UNIQUE,
		    name            TEXT NOT NULL,
		    slug            TEXT NOT NULL UNIQUE,
		    description     TEXT NOT NULL DEFAULT '',

		    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name);
		CREATE INDEX IF NOT EXISTS idx_cards_created_at ON cards(created_at);
		CREATE INDEX IF NOT EXISTS idx_cards_updated_at ON cards(updated_at);

		CREATE TABLE IF NOT EXISTS card_lists (
		    card_id         INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE ON UPDATE CASCADE,
		    list_id         INTEGER NOT NULL REFERENCES lists(id) ON DELETE CASCADE ON UPDATE CASCADE,
		    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

		    PRIMARY KEY(card_id, list_id)
		);
		CREATE INDEX IF NOT EXISTS idx_card_lists_list_id ON card_lists(list_id);
	`); err != nil {
		return err
	}

	return nil
}
