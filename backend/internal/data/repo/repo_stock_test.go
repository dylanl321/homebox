package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stockFixture(t *testing.T, quantity float64) (EntityOut, EntityOut, EntityOut) {
	t.Helper()
	ctx := context.Background()
	locationType := useContainerEntityType(t)
	itemType := useItemEntityType(t)

	first, err := tRepos.Entities.Create(ctx, tGroup.ID, EntityCreate{
		Name: "Stock location A " + uuid.NewString(), EntityTypeID: locationType.ID,
	})
	require.NoError(t, err)
	second, err := tRepos.Entities.Create(ctx, tGroup.ID, EntityCreate{
		Name: "Stock location B " + uuid.NewString(), EntityTypeID: locationType.ID,
	})
	require.NoError(t, err)
	item, err := tRepos.Entities.Create(ctx, tGroup.ID, EntityCreate{
		Name: "Stock item " + uuid.NewString(), EntityTypeID: itemType.ID,
		ParentID: first.ID, Quantity: quantity,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tRepos.Entities.DeleteByGroup(ctx, tGroup.ID, item.ID)
		_ = tRepos.Entities.DeleteByGroup(ctx, tGroup.ID, first.ID)
		_ = tRepos.Entities.DeleteByGroup(ctx, tGroup.ID, second.ID)
	})
	return first, second, item
}

func stockRequest(operation, key string) StockOperationRequest {
	return StockOperationRequest{Operation: operation, IdempotencyKey: key}
}

func TestStockAdjustSetTransferAndDefaultPromotion(t *testing.T) {
	ctx := context.Background()
	first, second, item := stockFixture(t, 10)

	adjust := stockRequest("adjust", uuid.NewString())
	adjust.LocationID = &second.ID
	delta := 4.0
	adjust.Delta = &delta
	state, _, err := tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, adjust)
	require.NoError(t, err)
	assert.InDelta(t, 14.0, state.TotalQuantity, 0.000001)
	assert.Len(t, state.Allocations, 2)

	set := stockRequest("set", uuid.NewString())
	set.LocationID = &second.ID
	exact := 3.5
	set.Quantity = &exact
	state, _, err = tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, set)
	require.NoError(t, err)
	assert.InDelta(t, 13.5, state.TotalQuantity, 0.000001)

	transfer := stockRequest(stockOperationTransfer, uuid.NewString())
	transfer.FromLocationID = &first.ID
	transfer.ToLocationID = &second.ID
	amount := 10.0
	transfer.Quantity = &amount
	state, _, err = tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, transfer)
	require.NoError(t, err)
	require.Len(t, state.Allocations, 1)
	assert.Equal(t, second.ID, *state.DefaultLocationID)
	assert.InDelta(t, 13.5, state.TotalQuantity, 0.000001)

	got, err := tRepos.Entities.GetOneByGroup(ctx, tGroup.ID, item.ID)
	require.NoError(t, err)
	assert.InDelta(t, state.TotalQuantity, got.Quantity, 0.000001)
	require.NotNil(t, got.Parent)
	assert.Equal(t, second.ID, got.Parent.ID)
}

func TestStockInsufficientAndIdempotency(t *testing.T) {
	ctx := context.Background()
	first, second, item := stockFixture(t, 5)

	transfer := stockRequest(stockOperationTransfer, uuid.NewString())
	transfer.FromLocationID = &first.ID
	transfer.ToLocationID = &second.ID
	tooMuch := 6.0
	transfer.Quantity = &tooMuch
	_, _, err := tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, transfer)
	require.Error(t, err)
	assert.True(t, IsStockErrorCode(err, "insufficient_stock"))

	adjust := stockRequest("adjust", uuid.NewString())
	adjust.LocationID = &first.ID
	delta := 2.0
	adjust.Delta = &delta
	firstState, firstTx, err := tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, adjust)
	require.NoError(t, err)
	replayed, replayTx, err := tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, adjust)
	require.NoError(t, err)
	assert.InDelta(t, firstState.TotalQuantity, replayed.TotalQuantity, 0.000001)
	assert.Equal(t, firstTx.ID, replayTx.ID)

	different := adjust
	otherDelta := 1.0
	different.Delta = &otherDelta
	_, _, err = tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, different)
	require.Error(t, err)
	assert.True(t, IsStockErrorCode(err, "idempotency_conflict"))
}

func TestStockEligibilityAndTenantLocationValidation(t *testing.T) {
	ctx := context.Background()
	first, second, item := stockFixture(t, 2)
	childType := useItemEntityType(t)
	child, err := tRepos.Entities.Create(ctx, tGroup.ID, EntityCreate{
		Name: "child " + uuid.NewString(), EntityTypeID: childType.ID,
		ParentID: item.ID, Quantity: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Entities.DeleteByGroup(ctx, tGroup.ID, child.ID) })

	req := stockRequest("adjust", uuid.NewString())
	req.LocationID = &second.ID
	delta := 1.0
	req.Delta = &delta
	_, _, err = tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, req)
	require.Error(t, err)
	assert.True(t, IsStockErrorCode(err, "container_item"))

	otherGroup, err := tRepos.Groups.GroupCreate(ctx, "other "+uuid.NewString(), uuid.Nil)
	require.NoError(t, err)
	locationType, err := tRepos.EntityTypes.GetDefault(ctx, otherGroup.ID, true)
	require.NoError(t, err)
	foreign, err := tRepos.Entities.Create(ctx, otherGroup.ID, EntityCreate{
		Name: "foreign", EntityTypeID: locationType.ID,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Entities.DeleteByGroup(ctx, otherGroup.ID, foreign.ID) })
	req.IdempotencyKey = uuid.NewString()
	req.LocationID = &foreign.ID
	_, _, err = tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, req)
	require.Error(t, err)
	assert.True(t, IsStockErrorCode(err, "invalid_location"))

	_ = first
}

func TestLegacyMultiLocationConflict(t *testing.T) {
	ctx := context.Background()
	_, second, item := stockFixture(t, 2)
	req := stockRequest("adjust", uuid.NewString())
	req.LocationID = &second.ID
	delta := 1.0
	req.Delta = &delta
	_, _, err := tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, item.ID, req)
	require.NoError(t, err)

	newQuantity := 10.0
	err = tRepos.Entities.Patch(ctx, tGroup.ID, item.ID, EntityPatch{Quantity: &newQuantity})
	require.Error(t, err)
	assert.True(t, IsStockErrorCode(err, "multi_location_stock"))
}

func TestLocationDeletionAndAtomicResolution(t *testing.T) {
	ctx := context.Background()
	first, second, item := stockFixture(t, 7)
	err := tRepos.Entities.DeleteByGroup(ctx, tGroup.ID, first.ID)
	require.Error(t, err)
	assert.True(t, IsStockErrorCode(err, "location_has_stock"))

	state, err := tRepos.Stock.LocationResolutionState(ctx, tGroup.ID, first.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, state.ItemCount)
	assert.InDelta(t, 7.0, state.TotalQuantity, 0.000001)

	result, err := tRepos.Stock.ResolveLocation(ctx, tGroup.ID, tUser.ID, first.ID, LocationStockResolutionRequest{
		Action: stockOperationTransfer, DestinationLocationID: &second.ID,
		IdempotencyKey: uuid.NewString(), Workflow: "test", Reason: "delete location",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.ItemCount)
	assert.InDelta(t, 7.0, result.TotalQuantity, 0.000001)

	itemState, err := tRepos.Stock.Get(ctx, tGroup.ID, item.ID)
	require.NoError(t, err)
	require.Len(t, itemState.Allocations, 1)
	assert.Equal(t, second.ID, *itemState.Allocations[0].LocationID)
	require.NoError(t, tRepos.Entities.DeleteByGroup(ctx, tGroup.ID, first.ID))
}

func TestLocationResolutionRollsBackAllItems(t *testing.T) {
	ctx := context.Background()
	first, second, firstItem := stockFixture(t, 3)
	itemType := useItemEntityType(t)
	secondItem, err := tRepos.Entities.Create(ctx, tGroup.ID, EntityCreate{
		Name: "second resolution item " + uuid.NewString(), EntityTypeID: itemType.ID,
		ParentID: first.ID, Quantity: 4,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Entities.DeleteByGroup(ctx, tGroup.ID, secondItem.ID) })

	key := uuid.NewString()
	seedDelta := 1.0
	_, _, err = tRepos.Stock.Operate(ctx, tGroup.ID, tUser.ID, firstItem.ID, StockOperationRequest{
		Operation: "adjust", LocationID: &first.ID, Delta: &seedDelta,
		IdempotencyKey: key + ":" + secondItem.ID.String(),
	})
	require.NoError(t, err)

	_, err = tRepos.Stock.ResolveLocation(ctx, tGroup.ID, tUser.ID, first.ID, LocationStockResolutionRequest{
		Action: stockOperationTransfer, DestinationLocationID: &second.ID,
		IdempotencyKey: key, Workflow: "test",
	})
	require.Error(t, err)

	firstState, err := tRepos.Stock.Get(ctx, tGroup.ID, firstItem.ID)
	require.NoError(t, err)
	secondState, err := tRepos.Stock.Get(ctx, tGroup.ID, secondItem.ID)
	require.NoError(t, err)
	assert.NotNil(t, findStockAllocation(firstState, first.ID))
	assert.NotNil(t, findStockAllocation(secondState, first.ID))
	assert.Nil(t, findStockAllocation(firstState, second.ID))
	assert.Nil(t, findStockAllocation(secondState, second.ID))
}

func findStockAllocation(state StockState, locationID uuid.UUID) *StockAllocation {
	for i := range state.Allocations {
		if state.Allocations[i].LocationID != nil && *state.Allocations[i].LocationID == locationID {
			return &state.Allocations[i]
		}
	}
	return nil
}
