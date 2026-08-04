package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/migrations"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	_ "github.com/sysadminsmedia/homebox/backend/pkgs/cgofreesqlite"
)

func TestStockMigrationBackfillsNearestLocation(t *testing.T) {
	ctx := context.Background()
	client, err := ent.Open("sqlite3", "file:stock-migration?mode=memory&cache=shared&_fk=1&_time_format=sqlite")
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	files, err := migrations.Migrations(config.DriverSqlite3)
	require.NoError(t, err)
	goose.SetBaseFS(files)
	require.NoError(t, goose.SetDialect(config.DriverSqlite3))
	require.NoError(t, goose.UpTo(client.Sql(), config.DriverSqlite3, 20260802000000))

	groupRow, err := client.Group.Create().SetName("migration").Save(ctx)
	require.NoError(t, err)
	locationType, err := client.EntityType.Create().
		SetName("Location").SetDescription("").SetIsLocation(true).SetGroupID(groupRow.ID).Save(ctx)
	require.NoError(t, err)
	itemType, err := client.EntityType.Create().
		SetName("Item").SetDescription("").SetIsLocation(false).SetGroupID(groupRow.ID).Save(ctx)
	require.NoError(t, err)
	location, err := client.Entity.Create().
		SetName("Room").SetDescription("").SetQuantity(0).SetGroupID(groupRow.ID).
		SetEntityTypeID(locationType.ID).Save(ctx)
	require.NoError(t, err)
	containerItem, err := client.Entity.Create().
		SetName("Toolbox").SetDescription("").SetQuantity(1).SetGroupID(groupRow.ID).
		SetEntityTypeID(itemType.ID).SetParentID(location.ID).Save(ctx)
	require.NoError(t, err)
	nested, err := client.Entity.Create().
		SetName("Bits").SetDescription("").SetQuantity(4.5).SetGroupID(groupRow.ID).
		SetEntityTypeID(itemType.ID).SetParentID(containerItem.ID).Save(ctx)
	require.NoError(t, err)
	unassigned, err := client.Entity.Create().
		SetName("Loose").SetDescription("").SetQuantity(2).SetGroupID(groupRow.ID).
		SetEntityTypeID(itemType.ID).Save(ctx)
	require.NoError(t, err)
	zero, err := client.Entity.Create().
		SetName("Empty").SetDescription("").SetQuantity(0).SetGroupID(groupRow.ID).
		SetEntityTypeID(itemType.ID).SetParentID(location.ID).Save(ctx)
	require.NoError(t, err)

	require.NoError(t, goose.Up(client.Sql(), config.DriverSqlite3))

	type allocation struct {
		EntityID   uuid.UUID
		LocationID *uuid.UUID
		Quantity   float64
		IsDefault  bool
	}
	queryRows, err := client.Sql().QueryContext(ctx,
		"SELECT entity_id, location_id, quantity, is_default FROM entity_stock_allocations")
	require.NoError(t, err)
	defer func() { _ = queryRows.Close() }()
	byEntity := make(map[uuid.UUID]allocation)
	for queryRows.Next() {
		var row allocation
		require.NoError(t, queryRows.Scan(&row.EntityID, &row.LocationID, &row.Quantity, &row.IsDefault))
		byEntity[row.EntityID] = row
	}
	require.NoError(t, queryRows.Err())
	require.Contains(t, byEntity, nested.ID)
	assert.Equal(t, location.ID, *byEntity[nested.ID].LocationID)
	assert.InDelta(t, 4.5, byEntity[nested.ID].Quantity, 0.000001)
	assert.True(t, byEntity[nested.ID].IsDefault)
	require.Contains(t, byEntity, unassigned.ID)
	assert.Nil(t, byEntity[unassigned.ID].LocationID)
	assert.NotContains(t, byEntity, zero.ID)
}
