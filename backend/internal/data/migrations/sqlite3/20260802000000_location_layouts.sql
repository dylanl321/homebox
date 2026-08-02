-- +goose Up
CREATE TABLE IF NOT EXISTS location_layouts (
    id                     uuid     NOT NULL PRIMARY KEY,
    created_at             datetime NOT NULL,
    updated_at             datetime NOT NULL,
    canvas_width           integer  NOT NULL DEFAULT 1000,
    canvas_height          integer  NOT NULL DEFAULT 700,
    revision               integer  NOT NULL DEFAULT 1 CHECK (revision > 0),
    entity_location_layout uuid     NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    CONSTRAINT location_layout_owner UNIQUE (entity_location_layout),
    CONSTRAINT location_layout_fixed_canvas CHECK (canvas_width = 1000 AND canvas_height = 700)
);

CREATE TABLE IF NOT EXISTS location_layout_elements (
    id                                      uuid     NOT NULL PRIMARY KEY,
    created_at                              datetime NOT NULL,
    updated_at                              datetime NOT NULL,
    kind                                    text     NOT NULL CHECK (kind IN ('wall', 'location')),
    x                                       real     NOT NULL,
    y                                       real     NOT NULL,
    width                                   real     NOT NULL DEFAULT 0,
    height                                  real     NOT NULL DEFAULT 0,
    end_x                                   real     NOT NULL DEFAULT 0,
    end_y                                   real     NOT NULL DEFAULT 0,
    rotation                                real     NOT NULL DEFAULT 0,
    z_order                                 integer  NOT NULL DEFAULT 0,
    location_layout_elements                 uuid     NOT NULL REFERENCES location_layouts(id) ON DELETE CASCADE,
    entity_layout_placements                uuid     REFERENCES entities(id) ON DELETE CASCADE,
    CONSTRAINT location_layout_element_target_unique UNIQUE (location_layout_elements, entity_layout_placements),
    CONSTRAINT location_layout_element_origin CHECK (x >= 0 AND x <= 1 AND y >= 0 AND y <= 1),
    CONSTRAINT location_layout_element_rotation CHECK (rotation >= -180 AND rotation <= 180),
    CONSTRAINT location_layout_wall_geometry CHECK (
        kind <> 'wall' OR (end_x >= 0 AND end_x <= 1 AND end_y >= 0 AND end_y <= 1 AND entity_layout_placements IS NULL)
    ),
    CONSTRAINT location_layout_target_geometry CHECK (
        kind <> 'location' OR (width > 0 AND height > 0 AND x + width <= 1 AND y + height <= 1 AND entity_layout_placements IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS locationlayoutelement_location_layout_elements
    ON location_layout_elements (location_layout_elements);
CREATE INDEX IF NOT EXISTS locationlayoutelement_entity_layout_placements
    ON location_layout_elements (entity_layout_placements);

-- +goose Down
DROP TABLE IF EXISTS location_layout_elements;
DROP TABLE IF EXISTS location_layouts;
