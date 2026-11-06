-- cards
-- name: query-cards
-- Lists (labels) are aggregated as JSON so a card is a single row.
WITH cards AS (
    SELECT
        COUNT(*) OVER () AS total,
        cards.*
    FROM cards
    WHERE ($1 = 0 OR cards.id = $1)
        AND ($2 = '' OR cards.slug = $2)
        AND ($3 = '' OR TO_TSVECTOR(cards.name) @@ PLAINTO_TSQUERY($3)
            OR cards.name ILIKE ('%' || $3 || '%')
            OR cards.slug ILIKE ('%' || $3 || '%'))
    ORDER BY %order% OFFSET $4 LIMIT (CASE WHEN $5 < 1 THEN NULL ELSE $5 END)
)
SELECT
    c.*,
    COALESCE((
        SELECT JSON_AGG(ROW_TO_JSON(l) ORDER BY l.name)
        FROM (
            SELECT lists.id, lists.uuid, lists.name, lists.type, lists.optin, lists.status, lists.description
            FROM lists
            INNER JOIN card_lists ON (card_lists.list_id = lists.id)
            WHERE card_lists.card_id = c.id
        ) l
    ), '[]') AS lists
FROM cards c ORDER BY %order%;

-- name: get-card
-- Get a single card by id, UUID, or slug along with its lists (labels).
SELECT
    c.*,
    COALESCE((
        SELECT JSON_AGG(ROW_TO_JSON(l) ORDER BY l.name)
        FROM (
            SELECT lists.id, lists.uuid, lists.name, lists.type, lists.optin, lists.status, lists.description
            FROM lists
            INNER JOIN card_lists ON (card_lists.list_id = lists.id)
            WHERE card_lists.card_id = c.id
        ) l
    ), '[]') AS lists
FROM cards c
WHERE CASE
    WHEN $1 > 0 THEN c.id = $1
    WHEN $2 != '' THEN c.uuid = $2::UUID
    WHEN $3 != '' THEN c.slug = $3
END;

-- name: card-slug-exists
-- Checks if a slug is taken, optionally ignoring the card being updated.
SELECT EXISTS(
    SELECT 1 FROM cards WHERE slug = $1 AND ($2 = 0 OR id != $2)
) AS exists;

-- name: create-card
-- Create a card and attach its lists (labels).
WITH c AS (
    INSERT INTO cards (uuid, name, slug, description)
        VALUES ($1, $2, $3, $4)
        RETURNING id
), l AS (
    INSERT INTO card_lists (card_id, list_id)
        SELECT (SELECT id FROM c), id FROM UNNEST($5::INT[]) AS id
        ON CONFLICT (card_id, list_id) DO NOTHING
)
SELECT id FROM c;

-- name: update-card
-- Update a card and reconcile its lists (labels).
WITH c AS (
    UPDATE cards SET
        name = $2,
        slug = $3,
        description = (CASE WHEN $4 != '' THEN $4 ELSE description END),
        updated_at = NOW()
    WHERE id = $1
    RETURNING id
), d AS (
    DELETE FROM card_lists
    WHERE card_id = (SELECT id FROM c) AND list_id != ALL($5::INT[])
), i AS (
    INSERT INTO card_lists (card_id, list_id)
        SELECT (SELECT id FROM c), id FROM UNNEST($5::INT[]) AS id
        ON CONFLICT (card_id, list_id) DO NOTHING
)
SELECT id FROM c;

-- name: delete-card
DELETE FROM cards WHERE id = $1;
