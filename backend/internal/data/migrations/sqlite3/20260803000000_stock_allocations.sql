-- +goose Up
CREATE TABLE entity_stock_allocations (
    id          uuid     NOT NULL PRIMARY KEY,
    created_at  datetime NOT NULL,
    updated_at  datetime NOT NULL,
    entity_id   uuid     NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    location_id uuid     REFERENCES entities(id) ON DELETE RESTRICT,
    quantity    real     NOT NULL CHECK (quantity > 0),
    is_default  boolean  NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX entity_stock_allocations_entity_location
    ON entity_stock_allocations(entity_id, location_id)
    WHERE location_id IS NOT NULL;
CREATE UNIQUE INDEX entity_stock_allocations_entity_unlocated
    ON entity_stock_allocations(entity_id)
    WHERE location_id IS NULL;
CREATE UNIQUE INDEX entity_stock_allocations_one_default
    ON entity_stock_allocations(entity_id)
    WHERE is_default = true;
CREATE INDEX entity_stock_allocations_location
    ON entity_stock_allocations(location_id);

CREATE TABLE entity_stock_transactions (
    id                      uuid     NOT NULL PRIMARY KEY,
    created_at              datetime NOT NULL,
    updated_at              datetime NOT NULL,
    group_id                uuid     NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    entity_id               uuid     NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    actor_id                uuid,
    operation               text     NOT NULL CHECK (operation IN ('adjust','set','transfer','resolve_transfer','resolve_remove','legacy')),
    workflow                text     NOT NULL DEFAULT '',
    source_location_id      uuid,
    destination_location_id uuid,
    quantity                real     NOT NULL,
    before_total            real     NOT NULL,
    after_total             real     NOT NULL,
    source_before           real,
    source_after            real,
    destination_before      real,
    destination_after       real,
    reason                  text     NOT NULL DEFAULT '',
    idempotency_key         text     NOT NULL,
    request_hash            text     NOT NULL,
    CONSTRAINT entity_stock_transactions_idempotency UNIQUE (group_id, idempotency_key)
);

CREATE INDEX entity_stock_transactions_group_created
    ON entity_stock_transactions(group_id, created_at);
CREATE INDEX entity_stock_transactions_entity_created
    ON entity_stock_transactions(entity_id, created_at);
CREATE INDEX entity_stock_transactions_source
    ON entity_stock_transactions(source_location_id);
CREATE INDEX entity_stock_transactions_destination
    ON entity_stock_transactions(destination_location_id);

WITH RECURSIVE ancestors(item_id, ancestor_id, parent_id, depth, is_location) AS (
    SELECT e.id, p.id, p.entity_children, 1, COALESCE(pet.is_location, false)
    FROM entities e
    LEFT JOIN entity_types et ON et.id = e.entity_type_entities
    JOIN entities p ON p.id = e.entity_children
    LEFT JOIN entity_types pet ON pet.id = p.entity_type_entities
    WHERE COALESCE(et.is_location, false) = false AND e.quantity > 0
    UNION ALL
    SELECT a.item_id, p.id, p.entity_children, a.depth + 1, COALESCE(pet.is_location, false)
    FROM ancestors a
    JOIN entities p ON p.id = a.parent_id
    LEFT JOIN entity_types pet ON pet.id = p.entity_type_entities
    WHERE a.depth < 64
),
nearest_locations AS (
    SELECT item_id, ancestor_id,
           ROW_NUMBER() OVER (PARTITION BY item_id ORDER BY depth) AS ordinal
    FROM ancestors
    WHERE is_location = true
)
INSERT INTO entity_stock_allocations
    (id, created_at, updated_at, entity_id, location_id, quantity, is_default)
SELECT
    lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
    substr(lower(hex(randomblob(2))), 2) || '-' ||
    substr('89ab', abs(random()) % 4 + 1, 1) ||
    substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6))),
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    e.id,
    nl.ancestor_id,
    e.quantity,
    true
FROM entities e
LEFT JOIN entity_types et ON et.id = e.entity_type_entities
LEFT JOIN nearest_locations nl ON nl.item_id = e.id AND nl.ordinal = 1
WHERE COALESCE(et.is_location, false) = false AND e.quantity > 0;

-- +goose Down
DROP TABLE IF EXISTS entity_stock_transactions;
DROP TABLE IF EXISTS entity_stock_allocations;
