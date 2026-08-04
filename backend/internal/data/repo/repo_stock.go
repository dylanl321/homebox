package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/reporting/eventbus"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entity"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entitystockallocation"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entitystocktransaction"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entitytype"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/predicate"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/user"
)

const (
	stockEpsilon           = 1e-9
	stockOperationTransfer = "transfer"
)

type StockError struct {
	Code string
	Err  error
}

func (e *StockError) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *StockError) Unwrap() error { return e.Err }

func IsStockErrorCode(err error, code string) bool {
	var stockErr *StockError
	return errors.As(err, &stockErr) && stockErr.Code == code
}

func stockError(code, message string) error {
	return &StockError{Code: code, Err: errors.New(message)}
}

type StockAllocation struct {
	ID         uuid.UUID      `json:"id"`
	ItemID     uuid.UUID      `json:"itemId"`
	LocationID *uuid.UUID     `json:"locationId"         extensions:"x-nullable"`
	Location   *EntitySummary `json:"location,omitempty" extensions:"x-nullable,x-omitempty"`
	Quantity   float64        `json:"quantity"`
	IsDefault  bool           `json:"isDefault"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

type StockState struct {
	TotalQuantity     float64           `json:"totalQuantity"`
	DefaultLocationID *uuid.UUID        `json:"defaultLocationId" extensions:"x-nullable"`
	Allocations       []StockAllocation `json:"allocations"`
}

type StockOperationRequest struct {
	Operation      string     `json:"operation"                validate:"required,oneof=adjust set transfer"`
	LocationID     *uuid.UUID `json:"locationId,omitempty"     extensions:"x-nullable,x-omitempty"`
	FromLocationID *uuid.UUID `json:"fromLocationId,omitempty" extensions:"x-nullable,x-omitempty"`
	ToLocationID   *uuid.UUID `json:"toLocationId,omitempty"   extensions:"x-nullable,x-omitempty"`
	Delta          *float64   `json:"delta,omitempty"          extensions:"x-nullable,x-omitempty"`
	Quantity       *float64   `json:"quantity,omitempty"       extensions:"x-nullable,x-omitempty"`
	SetDefault     bool       `json:"setDefault,omitempty"`
	Workflow       string     `json:"workflow,omitempty"       validate:"max=100"`
	Reason         string     `json:"reason,omitempty"         validate:"max=1000"`
	IdempotencyKey string     `json:"idempotencyKey"           validate:"required,min=1,max=255"`
}

type StockTransaction struct {
	ID                    uuid.UUID             `json:"id"`
	EntityID              uuid.UUID             `json:"entityId"`
	ActorID               *uuid.UUID            `json:"actorId,omitempty"               extensions:"x-nullable,x-omitempty"`
	ActorName             string                `json:"actorName,omitempty"`
	Operation             string                `json:"operation"`
	Workflow              string                `json:"workflow"`
	SourceLocationID      *uuid.UUID            `json:"sourceLocationId,omitempty"      extensions:"x-nullable,x-omitempty"`
	SourceLocation        *StockLocationSummary `json:"sourceLocation,omitempty"        extensions:"x-nullable,x-omitempty"`
	DestinationLocationID *uuid.UUID            `json:"destinationLocationId,omitempty" extensions:"x-nullable,x-omitempty"`
	DestinationLocation   *StockLocationSummary `json:"destinationLocation,omitempty"   extensions:"x-nullable,x-omitempty"`
	Quantity              float64               `json:"quantity"`
	BeforeTotal           float64               `json:"beforeTotal"`
	AfterTotal            float64               `json:"afterTotal"`
	SourceBefore          *float64              `json:"sourceBefore,omitempty"          extensions:"x-nullable,x-omitempty"`
	SourceAfter           *float64              `json:"sourceAfter,omitempty"           extensions:"x-nullable,x-omitempty"`
	DestinationBefore     *float64              `json:"destinationBefore,omitempty"     extensions:"x-nullable,x-omitempty"`
	DestinationAfter      *float64              `json:"destinationAfter,omitempty"      extensions:"x-nullable,x-omitempty"`
	Reason                string                `json:"reason"`
	IdempotencyKey        string                `json:"idempotencyKey"`
	CreatedAt             time.Time             `json:"createdAt"`
}

type StockLocationSummary struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type StockTransactionQuery struct {
	EntityID   *uuid.UUID
	LocationID *uuid.UUID
	Page       int
	PageSize   int
}

type LocationStockResolutionRequest struct {
	Action                string     `json:"action"                          validate:"required,oneof=transfer remove"`
	DestinationLocationID *uuid.UUID `json:"destinationLocationId,omitempty" extensions:"x-nullable,x-omitempty"`
	Confirmed             bool       `json:"confirmed"`
	Workflow              string     `json:"workflow,omitempty"              validate:"max=100"`
	Reason                string     `json:"reason,omitempty"                validate:"max=1000"`
	IdempotencyKey        string     `json:"idempotencyKey"                  validate:"required,min=1,max=255"`
}

type LocationStockConflict struct {
	EntityID   uuid.UUID `json:"entityId"`
	EntityName string    `json:"entityName"`
	Quantity   float64   `json:"quantity"`
	IsDefault  bool      `json:"isDefault"`
}

type LocationStockResolutionResult struct {
	LocationID    uuid.UUID               `json:"locationId"`
	ItemCount     int                     `json:"itemCount"`
	TotalQuantity float64                 `json:"totalQuantity"`
	Allocations   []LocationStockConflict `json:"allocations"`
}

type SetDefaultStockRequest struct {
	LocationID *uuid.UUID `json:"locationId,omitempty" extensions:"x-nullable,x-omitempty"`
}

type StockImportAllocation struct {
	LocationID *uuid.UUID
	Quantity   float64
	IsDefault  bool
}

type StockRepository struct {
	db  *ent.Client
	bus *eventbus.EventBus
}

func NewStockRepository(db *ent.Client, bus *eventbus.EventBus) *StockRepository {
	return &StockRepository{db: db, bus: bus}
}

func mapStockAllocation(row *ent.EntityStockAllocation) StockAllocation {
	var location *EntitySummary
	if row.Edges.Location != nil {
		value := mapEntitySummary(row.Edges.Location)
		location = &value
	}
	return StockAllocation{
		ID:         row.ID,
		ItemID:     row.EntityID,
		LocationID: row.LocationID,
		Location:   location,
		Quantity:   row.Quantity,
		IsDefault:  row.IsDefault,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}

func stockStateFromRows(rows []*ent.EntityStockAllocation) StockState {
	state := StockState{Allocations: make([]StockAllocation, 0, len(rows))}
	for _, row := range rows {
		state.TotalQuantity += row.Quantity
		state.Allocations = append(state.Allocations, mapStockAllocation(row))
		if row.IsDefault {
			state.DefaultLocationID = row.LocationID
		}
	}
	if math.Abs(state.TotalQuantity) < stockEpsilon {
		state.TotalQuantity = 0
	}
	return state
}

func allocationLocationPredicate(locationID *uuid.UUID) predicate.EntityStockAllocation {
	if locationID == nil {
		return entitystockallocation.LocationIDIsNil()
	}
	return entitystockallocation.LocationID(*locationID)
}

func sameLocation(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func validFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (r *StockRepository) queryRows(ctx context.Context, client *ent.Client, entityID uuid.UUID, lock bool) ([]*ent.EntityStockAllocation, error) {
	build := func() *ent.EntityStockAllocationQuery {
		return client.EntityStockAllocation.Query().
			Where(entitystockallocation.EntityID(entityID)).
			WithLocation(func(eq *ent.EntityQuery) {
				eq.WithEntityType()
			}).
			Order(entitystockallocation.ByCreatedAt(), entitystockallocation.ByID())
	}
	q := build()
	if lock {
		q.Where(func(s *entsql.Selector) { s.ForUpdate() })
	}
	rows, err := q.All(ctx)
	if lock && err != nil && strings.Contains(err.Error(), "FOR UPDATE/SHARE not supported in SQLite") {
		return build().All(ctx)
	}
	return rows, err
}

func (r *StockRepository) Get(ctx context.Context, gid, entityID uuid.UUID) (StockState, error) {
	exists, err := r.db.Entity.Query().
		Where(entity.ID(entityID), entity.HasGroupWith(group.ID(gid))).
		Exist(ctx)
	if err != nil {
		return StockState{}, err
	}
	if !exists {
		return StockState{}, &ent.NotFoundError{}
	}
	rows, err := r.queryRows(ctx, r.db, entityID, false)
	if err != nil {
		return StockState{}, err
	}
	return stockStateFromRows(rows), nil
}

func hashStockRequest(entityID uuid.UUID, request StockOperationRequest) (string, error) {
	payload := struct {
		EntityID uuid.UUID             `json:"entityId"`
		Request  StockOperationRequest `json:"request"`
	}{EntityID: entityID, Request: request}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func validateStockRequest(request StockOperationRequest) error {
	request.Operation = strings.ToLower(strings.TrimSpace(request.Operation))
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return stockError("idempotency_key_required", "idempotencyKey is required")
	}
	switch request.Operation {
	case "adjust":
		if request.Delta == nil || !validFinite(*request.Delta) {
			return stockError("invalid_quantity", "adjust delta must be finite")
		}
	case "set":
		if request.Quantity == nil || !validFinite(*request.Quantity) || *request.Quantity < 0 {
			return stockError("invalid_quantity", "set quantity must be finite and non-negative")
		}
	case stockOperationTransfer:
		if request.Quantity == nil || !validFinite(*request.Quantity) || *request.Quantity <= 0 {
			return stockError("invalid_quantity", "transfer quantity must be finite and positive")
		}
		if sameLocation(request.FromLocationID, request.ToLocationID) {
			return stockError("same_location", "source and destination locations must differ")
		}
	default:
		return stockError("invalid_operation", "operation must be adjust, set, or transfer")
	}
	return nil
}

func (r *StockRepository) validateLocation(ctx context.Context, client *ent.Client, gid uuid.UUID, locationID *uuid.UUID) error {
	if locationID == nil {
		return nil
	}
	ok, err := client.Entity.Query().
		Where(
			entity.ID(*locationID),
			entity.HasGroupWith(group.ID(gid)),
			entity.HasEntityTypeWith(entitytype.IsLocation(true)),
		).
		Exist(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return stockError("invalid_location", "location does not exist in this collection")
	}
	return nil
}

func (r *StockRepository) ensureMultiLocationEligible(item *ent.Entity, rows []*ent.EntityStockAllocation, target *uuid.UUID) error {
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if sameLocation(row.LocationID, target) {
			return nil
		}
	}
	if len(item.Edges.Children) > 0 {
		return stockError("container_item", "items containing children cannot use multiple stock locations")
	}
	if item.Edges.Parent == nil || item.Edges.Parent.Edges.EntityType == nil || !item.Edges.Parent.Edges.EntityType.IsLocation {
		return stockError("nested_item", "nested or unlocated items cannot use multiple stock locations")
	}
	return nil
}

func findAllocation(rows []*ent.EntityStockAllocation, locationID *uuid.UUID) *ent.EntityStockAllocation {
	for _, row := range rows {
		if sameLocation(row.LocationID, locationID) {
			return row
		}
	}
	return nil
}

func setFloat(value float64) *float64 { return &value }

func (r *StockRepository) saveAllocation(ctx context.Context, client *ent.Client, item *ent.Entity, rows []*ent.EntityStockAllocation, locationID *uuid.UUID, quantity float64) error {
	if quantity < -stockEpsilon || !validFinite(quantity) {
		return stockError("insufficient_stock", "operation would make allocated stock negative")
	}
	existing := findAllocation(rows, locationID)
	if quantity <= stockEpsilon {
		if existing != nil {
			return client.EntityStockAllocation.DeleteOneID(existing.ID).Exec(ctx)
		}
		return nil
	}
	if existing != nil {
		return client.EntityStockAllocation.UpdateOneID(existing.ID).SetQuantity(quantity).Exec(ctx)
	}
	if err := r.ensureMultiLocationEligible(item, rows, locationID); err != nil {
		return err
	}
	return client.EntityStockAllocation.Create().
		SetEntityID(item.ID).
		SetNillableLocationID(locationID).
		SetQuantity(quantity).
		SetIsDefault(len(rows) == 0).
		Exec(ctx)
}

func (r *StockRepository) promoteAndSync(ctx context.Context, client *ent.Client, item *ent.Entity) (StockState, error) {
	rows, err := r.queryRows(ctx, client, item.ID, false)
	if err != nil {
		return StockState{}, err
	}
	hasDefault := false
	for _, row := range rows {
		hasDefault = hasDefault || row.IsDefault
	}
	if len(rows) > 0 && !hasDefault {
		if err := client.EntityStockAllocation.UpdateOneID(rows[0].ID).SetIsDefault(true).Exec(ctx); err != nil {
			return StockState{}, err
		}
		rows[0].IsDefault = true
	}

	state := stockStateFromRows(rows)
	update := client.Entity.UpdateOneID(item.ID).SetQuantity(state.TotalQuantity)
	// Preserve item-in-item nesting. Leaf items directly under a location use
	// the default allocation as their compatibility parent.
	if item.Edges.Parent == nil || (item.Edges.Parent.Edges.EntityType != nil && item.Edges.Parent.Edges.EntityType.IsLocation) {
		if state.DefaultLocationID == nil {
			update.ClearParent()
		} else {
			update.SetParentID(*state.DefaultLocationID)
		}
	}
	if err := update.Exec(ctx); err != nil {
		return StockState{}, err
	}
	return state, nil
}

func (r *StockRepository) setDefault(ctx context.Context, client *ent.Client, entityID uuid.UUID, locationID *uuid.UUID) error {
	target, err := client.EntityStockAllocation.Query().
		Where(entitystockallocation.EntityID(entityID), allocationLocationPredicate(locationID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return stockError("allocation_not_found", "cannot set an empty location as default")
		}
		return err
	}
	if _, err := client.EntityStockAllocation.Update().
		Where(entitystockallocation.EntityID(entityID), entitystockallocation.IsDefault(true)).
		SetIsDefault(false).
		Save(ctx); err != nil {
		return err
	}
	return client.EntityStockAllocation.UpdateOneID(target.ID).SetIsDefault(true).Exec(ctx)
}

func mapStockTransaction(row *ent.EntityStockTransaction) StockTransaction {
	return StockTransaction{
		ID:                    row.ID,
		EntityID:              row.EntityID,
		ActorID:               row.ActorID,
		Operation:             row.Operation.String(),
		Workflow:              row.Workflow,
		SourceLocationID:      row.SourceLocationID,
		DestinationLocationID: row.DestinationLocationID,
		Quantity:              row.Quantity,
		BeforeTotal:           row.BeforeTotal,
		AfterTotal:            row.AfterTotal,
		SourceBefore:          row.SourceBefore,
		SourceAfter:           row.SourceAfter,
		DestinationBefore:     row.DestinationBefore,
		DestinationAfter:      row.DestinationAfter,
		Reason:                row.Reason,
		IdempotencyKey:        row.IdempotencyKey,
		CreatedAt:             row.CreatedAt,
	}
}

//nolint:gocyclo // Stock mutations deliberately keep validation, ledger writes, and cache updates in one transaction.
func (r *StockRepository) Operate(ctx context.Context, gid, actorID, entityID uuid.UUID, request StockOperationRequest) (StockState, StockTransaction, error) {
	request.Operation = strings.ToLower(strings.TrimSpace(request.Operation))
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if err := validateStockRequest(request); err != nil {
		return StockState{}, StockTransaction{}, err
	}
	requestHash, err := hashStockRequest(entityID, request)
	if err != nil {
		return StockState{}, StockTransaction{}, err
	}

	// Fast replay path.
	existing, err := r.db.EntityStockTransaction.Query().
		Where(entitystocktransaction.GroupID(gid), entitystocktransaction.IdempotencyKey(request.IdempotencyKey)).
		Only(ctx)
	if err == nil {
		if existing.RequestHash != requestHash {
			return StockState{}, StockTransaction{}, stockError("idempotency_conflict", "idempotency key was already used with a different request")
		}
		state, stateErr := r.Get(ctx, gid, entityID)
		return state, mapStockTransaction(existing), stateErr
	}
	if !ent.IsNotFound(err) {
		return StockState{}, StockTransaction{}, err
	}

	tx, err := r.db.Tx(ctx)
	if err != nil {
		return StockState{}, StockTransaction{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	client := tx.Client()

	buildItemQuery := func() *ent.EntityQuery {
		return client.Entity.Query().
			Where(entity.ID(entityID), entity.HasGroupWith(group.ID(gid))).
			WithEntityType().
			WithParent(func(q *ent.EntityQuery) { q.WithEntityType() }).
			WithChildren()
	}
	item, err := buildItemQuery().
		Where(func(s *entsql.Selector) { s.ForUpdate() }).
		Only(ctx)
	if err != nil && strings.Contains(err.Error(), "FOR UPDATE/SHARE not supported in SQLite") {
		item, err = buildItemQuery().Only(ctx)
	}
	if err != nil {
		return StockState{}, StockTransaction{}, err
	}
	if item.Edges.EntityType != nil && item.Edges.EntityType.IsLocation {
		return StockState{}, StockTransaction{}, stockError("location_not_stockable", "locations cannot have stock allocations")
	}

	for _, locationID := range []*uuid.UUID{request.LocationID, request.FromLocationID, request.ToLocationID} {
		if err := r.validateLocation(ctx, client, gid, locationID); err != nil {
			return StockState{}, StockTransaction{}, err
		}
	}
	rows, err := r.queryRows(ctx, client, entityID, true)
	if err != nil {
		return StockState{}, StockTransaction{}, err
	}
	beforeState := stockStateFromRows(rows)
	beforeTotal := beforeState.TotalQuantity

	var (
		sourceID, destinationID             *uuid.UUID
		sourceBefore, sourceAfter           *float64
		destinationBefore, destinationAfter *float64
		ledgerQuantity                      float64
	)

	switch request.Operation {
	case "adjust":
		current := findAllocation(rows, request.LocationID)
		before := 0.0
		if current != nil {
			before = current.Quantity
		}
		after := before + *request.Delta
		if after < -stockEpsilon {
			return StockState{}, StockTransaction{}, stockError("insufficient_stock", "insufficient stock at source location")
		}
		if err := r.saveAllocation(ctx, client, item, rows, request.LocationID, after); err != nil {
			return StockState{}, StockTransaction{}, err
		}
		ledgerQuantity = *request.Delta
		if *request.Delta < 0 {
			sourceID, sourceBefore, sourceAfter = request.LocationID, setFloat(before), setFloat(math.Max(after, 0))
		} else {
			destinationID, destinationBefore, destinationAfter = request.LocationID, setFloat(before), setFloat(after)
		}
	case "set":
		current := findAllocation(rows, request.LocationID)
		before := 0.0
		if current != nil {
			before = current.Quantity
		}
		if err := r.saveAllocation(ctx, client, item, rows, request.LocationID, *request.Quantity); err != nil {
			return StockState{}, StockTransaction{}, err
		}
		ledgerQuantity = *request.Quantity - before
		if ledgerQuantity < 0 {
			sourceID, sourceBefore, sourceAfter = request.LocationID, setFloat(before), setFloat(*request.Quantity)
		} else {
			destinationID, destinationBefore, destinationAfter = request.LocationID, setFloat(before), setFloat(*request.Quantity)
		}
	case stockOperationTransfer:
		source := findAllocation(rows, request.FromLocationID)
		if source == nil || source.Quantity+stockEpsilon < *request.Quantity {
			return StockState{}, StockTransaction{}, stockError("insufficient_stock", "insufficient stock at source location")
		}
		destination := findAllocation(rows, request.ToLocationID)
		destinationQuantity := 0.0
		if destination != nil {
			destinationQuantity = destination.Quantity
		}
		if err := r.ensureMultiLocationEligible(item, rows, request.ToLocationID); err != nil {
			return StockState{}, StockTransaction{}, err
		}
		if err := r.saveAllocation(ctx, client, item, rows, request.FromLocationID, source.Quantity-*request.Quantity); err != nil {
			return StockState{}, StockTransaction{}, err
		}
		// Reload because source removal can change the row set used by create.
		rows, err = r.queryRows(ctx, client, entityID, false)
		if err != nil {
			return StockState{}, StockTransaction{}, err
		}
		if err := r.saveAllocation(ctx, client, item, rows, request.ToLocationID, destinationQuantity+*request.Quantity); err != nil {
			return StockState{}, StockTransaction{}, err
		}
		ledgerQuantity = *request.Quantity
		sourceID, destinationID = request.FromLocationID, request.ToLocationID
		sourceBefore, sourceAfter = setFloat(source.Quantity), setFloat(source.Quantity-*request.Quantity)
		destinationBefore, destinationAfter = setFloat(destinationQuantity), setFloat(destinationQuantity+*request.Quantity)
	}

	if request.SetDefault {
		defaultLocation := request.LocationID
		if request.Operation == stockOperationTransfer {
			defaultLocation = request.ToLocationID
		}
		if err := r.setDefault(ctx, client, entityID, defaultLocation); err != nil {
			return StockState{}, StockTransaction{}, err
		}
	}
	afterState, err := r.promoteAndSync(ctx, client, item)
	if err != nil {
		return StockState{}, StockTransaction{}, err
	}

	builder := client.EntityStockTransaction.Create().
		SetGroupID(gid).
		SetEntityID(entityID).
		SetOperation(entitystocktransaction.Operation(request.Operation)).
		SetWorkflow(request.Workflow).
		SetNillableSourceLocationID(sourceID).
		SetNillableDestinationLocationID(destinationID).
		SetQuantity(ledgerQuantity).
		SetBeforeTotal(beforeTotal).
		SetAfterTotal(afterState.TotalQuantity).
		SetNillableSourceBefore(sourceBefore).
		SetNillableSourceAfter(sourceAfter).
		SetNillableDestinationBefore(destinationBefore).
		SetNillableDestinationAfter(destinationAfter).
		SetReason(request.Reason).
		SetIdempotencyKey(request.IdempotencyKey).
		SetRequestHash(requestHash)
	if actorID != uuid.Nil {
		builder.SetActorID(actorID)
	}
	ledger, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			_ = tx.Rollback()
			committed = true
			existing, findErr := r.db.EntityStockTransaction.Query().
				Where(entitystocktransaction.GroupID(gid), entitystocktransaction.IdempotencyKey(request.IdempotencyKey)).
				Only(ctx)
			if findErr == nil && existing.RequestHash == requestHash {
				state, stateErr := r.Get(ctx, gid, entityID)
				return state, mapStockTransaction(existing), stateErr
			}
			if findErr == nil {
				return StockState{}, StockTransaction{}, stockError("idempotency_conflict", "idempotency key was already used with a different request")
			}
		}
		return StockState{}, StockTransaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return StockState{}, StockTransaction{}, err
	}
	committed = true
	if r.bus != nil {
		r.bus.Publish(eventbus.EventEntityMutation, eventbus.GroupMutationEvent{GID: gid})
	}
	return afterState, mapStockTransaction(ledger), nil
}

func (r *StockRepository) SetDefault(ctx context.Context, gid, entityID uuid.UUID, locationID *uuid.UUID) (StockState, error) {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return StockState{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	client := tx.Client()
	item, err := client.Entity.Query().
		Where(entity.ID(entityID), entity.HasGroupWith(group.ID(gid))).
		WithParent(func(q *ent.EntityQuery) { q.WithEntityType() }).
		Only(ctx)
	if err != nil {
		return StockState{}, err
	}
	if err := r.setDefault(ctx, client, entityID, locationID); err != nil {
		return StockState{}, err
	}
	state, err := r.promoteAndSync(ctx, client, item)
	if err != nil {
		return StockState{}, err
	}
	if err := tx.Commit(); err != nil {
		return StockState{}, err
	}
	committed = true
	if r.bus != nil {
		r.bus.Publish(eventbus.EventEntityMutation, eventbus.GroupMutationEvent{GID: gid})
	}
	return state, nil
}

func (r *StockRepository) Transactions(ctx context.Context, gid uuid.UUID, query StockTransactionQuery) (PaginationResult[StockTransaction], error) {
	q := r.db.EntityStockTransaction.Query().Where(entitystocktransaction.GroupID(gid))
	if query.EntityID != nil {
		q.Where(entitystocktransaction.EntityID(*query.EntityID))
	}
	if query.LocationID != nil {
		q.Where(entitystocktransaction.Or(
			entitystocktransaction.SourceLocationID(*query.LocationID),
			entitystocktransaction.DestinationLocationID(*query.LocationID),
		))
	}
	count, err := q.Clone().Count(ctx)
	if err != nil {
		return PaginationResult[StockTransaction]{}, err
	}
	page, pageSize := query.Page, query.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if page < 0 {
		page = 0
	}
	rows, err := q.Order(ent.Desc(entitystocktransaction.FieldCreatedAt)).
		Offset(calculateOffset(page, pageSize)).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return PaginationResult[StockTransaction]{}, err
	}
	items := make([]StockTransaction, len(rows))
	locationIDs := make(map[uuid.UUID]struct{})
	actorIDs := make(map[uuid.UUID]struct{})
	for _, row := range rows {
		if row.SourceLocationID != nil {
			locationIDs[*row.SourceLocationID] = struct{}{}
		}
		if row.DestinationLocationID != nil {
			locationIDs[*row.DestinationLocationID] = struct{}{}
		}
		if row.ActorID != nil {
			actorIDs[*row.ActorID] = struct{}{}
		}
	}
	locationMap := make(map[uuid.UUID]StockLocationSummary, len(locationIDs))
	if len(locationIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(locationIDs))
		for id := range locationIDs {
			ids = append(ids, id)
		}
		locations, err := r.db.Entity.Query().
			Where(entity.IDIn(ids...), entity.HasGroupWith(group.ID(gid))).
			All(ctx)
		if err != nil {
			return PaginationResult[StockTransaction]{}, err
		}
		for _, location := range locations {
			locationMap[location.ID] = StockLocationSummary{ID: location.ID, Name: location.Name}
		}
	}
	actorMap := make(map[uuid.UUID]string, len(actorIDs))
	if len(actorIDs) > 0 {
		ids := make([]uuid.UUID, 0, len(actorIDs))
		for id := range actorIDs {
			ids = append(ids, id)
		}
		actors, err := r.db.User.Query().Where(user.IDIn(ids...)).All(ctx)
		if err != nil {
			return PaginationResult[StockTransaction]{}, err
		}
		for _, actor := range actors {
			actorMap[actor.ID] = actor.Name
		}
	}
	for i, row := range rows {
		items[i] = mapStockTransaction(row)
		if row.ActorID != nil {
			items[i].ActorName = actorMap[*row.ActorID]
		}
		if row.SourceLocationID != nil {
			if location, ok := locationMap[*row.SourceLocationID]; ok {
				items[i].SourceLocation = &location
			}
		}
		if row.DestinationLocationID != nil {
			if location, ok := locationMap[*row.DestinationLocationID]; ok {
				items[i].DestinationLocation = &location
			}
		}
	}
	return PaginationResult[StockTransaction]{Page: page, PageSize: pageSize, Total: count, Items: items}, nil
}

func (r *StockRepository) LocationConflicts(ctx context.Context, gid, locationID uuid.UUID) ([]LocationStockConflict, error) {
	if err := r.validateLocation(ctx, r.db, gid, &locationID); err != nil {
		return nil, err
	}
	rows, err := r.db.EntityStockAllocation.Query().
		Where(
			entitystockallocation.LocationID(locationID),
			entitystockallocation.QuantityGT(0),
			entitystockallocation.HasEntityWith(entity.HasGroupWith(group.ID(gid))),
		).
		WithEntity().
		Order(entitystockallocation.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LocationStockConflict, len(rows))
	for i, row := range rows {
		out[i] = LocationStockConflict{
			EntityID:   row.EntityID,
			EntityName: row.Edges.Entity.Name,
			Quantity:   row.Quantity,
			IsDefault:  row.IsDefault,
		}
	}
	return out, nil
}

func (r *StockRepository) LocationResolutionState(ctx context.Context, gid, locationID uuid.UUID) (LocationStockResolutionResult, error) {
	allocations, err := r.LocationConflicts(ctx, gid, locationID)
	if err != nil {
		return LocationStockResolutionResult{}, err
	}
	out := LocationStockResolutionResult{
		LocationID:  locationID,
		ItemCount:   len(allocations),
		Allocations: allocations,
	}
	for _, allocation := range allocations {
		out.TotalQuantity += allocation.Quantity
	}
	return out, nil
}

// EnsureInitial creates the compatibility allocation for a newly-created item.
func (r *StockRepository) ensureInitialClient(ctx context.Context, client *ent.Client, gid, entityID uuid.UUID, quantity float64) error {
	if !validFinite(quantity) || quantity < 0 {
		return stockError("invalid_quantity", "quantity must be finite and non-negative")
	}
	if quantity <= stockEpsilon {
		return nil
	}
	item, err := client.Entity.Query().
		Where(entity.ID(entityID), entity.HasGroupWith(group.ID(gid))).
		WithEntityType().
		WithParent(func(q *ent.EntityQuery) { q.WithEntityType() }).
		Only(ctx)
	if err != nil {
		return err
	}
	if item.Edges.EntityType != nil && item.Edges.EntityType.IsLocation {
		return nil
	}
	var locationID *uuid.UUID
	if item.Edges.Parent != nil {
		location, err := nearestLocationAncestor(ctx, client.Entity, item.Edges.Parent)
		if err != nil {
			return err
		}
		if location != nil {
			locationID = &location.ID
		}
	}
	exists, err := client.EntityStockAllocation.Query().
		Where(entitystockallocation.EntityID(entityID)).
		Exist(ctx)
	if err != nil || exists {
		return err
	}
	return client.EntityStockAllocation.Create().
		SetEntityID(entityID).
		SetNillableLocationID(locationID).
		SetQuantity(quantity).
		SetIsDefault(true).
		Exec(ctx)
}

func (r *StockRepository) EnsureInitial(ctx context.Context, gid, entityID uuid.UUID, quantity float64) error {
	return r.ensureInitialClient(ctx, r.db, gid, entityID, quantity)
}

// SyncLegacy applies a legacy quantity/location edit to an item that has at
// most one allocation. Multi-location items must use Operate.
func (r *StockRepository) syncLegacyClient(ctx context.Context, client *ent.Client, gid, entityID uuid.UUID, quantity float64, parentID uuid.UUID) error {
	if !validFinite(quantity) || quantity < 0 {
		return stockError("invalid_quantity", "quantity must be finite and non-negative")
	}
	rows, err := client.EntityStockAllocation.Query().
		Where(entitystockallocation.EntityID(entityID)).
		Order(entitystockallocation.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return err
	}
	if len(rows) > 1 {
		return stockError("multi_location_stock", "quantity and location changes on multi-location items must use the stock endpoint")
	}
	var locationID *uuid.UUID
	if parentID != uuid.Nil {
		parent, err := client.Entity.Query().
			Where(entity.ID(parentID), entity.HasGroupWith(group.ID(gid))).
			WithEntityType().
			Only(ctx)
		if err != nil {
			return err
		}
		if parent.Edges.EntityType != nil && parent.Edges.EntityType.IsLocation {
			locationID = &parent.ID
		} else {
			location, err := nearestLocationAncestor(ctx, client.Entity, parent)
			if err != nil {
				return err
			}
			if location != nil {
				locationID = &location.ID
			}
		}
	}
	if len(rows) == 1 {
		if quantity <= stockEpsilon {
			return client.EntityStockAllocation.DeleteOneID(rows[0].ID).Exec(ctx)
		}
		update := client.EntityStockAllocation.UpdateOneID(rows[0].ID).
			SetQuantity(quantity).
			SetIsDefault(true)
		if locationID == nil {
			update.ClearLocationID()
		} else {
			update.SetLocationID(*locationID)
		}
		return update.Exec(ctx)
	}
	if quantity <= stockEpsilon {
		return nil
	}
	return client.EntityStockAllocation.Create().
		SetEntityID(entityID).
		SetNillableLocationID(locationID).
		SetQuantity(quantity).
		SetIsDefault(true).
		Exec(ctx)
}

func (r *StockRepository) SyncLegacy(ctx context.Context, gid, entityID uuid.UUID, quantity float64, parentID uuid.UUID) error {
	return r.syncLegacyClient(ctx, r.db, gid, entityID, quantity, parentID)
}

func (r *StockRepository) ReplaceForImport(ctx context.Context, gid, entityID uuid.UUID, allocations []StockImportAllocation) error {
	tx, err := r.db.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	client := tx.Client()
	item, err := client.Entity.Query().
		Where(entity.ID(entityID), entity.HasGroupWith(group.ID(gid))).
		WithParent(func(q *ent.EntityQuery) { q.WithEntityType() }).
		WithChildren().
		Only(ctx)
	if err != nil {
		return err
	}
	defaultCount := 0
	for i := range allocations {
		if !validFinite(allocations[i].Quantity) || allocations[i].Quantity < 0 {
			return stockError("invalid_quantity", "import allocation quantity must be finite and non-negative")
		}
		if err := r.validateLocation(ctx, client, gid, allocations[i].LocationID); err != nil {
			return err
		}
		if allocations[i].IsDefault && allocations[i].Quantity > stockEpsilon {
			defaultCount++
		}
	}
	if defaultCount > 1 {
		return stockError("invalid_default", "only one imported allocation may be default")
	}
	if _, err := client.EntityStockAllocation.Delete().
		Where(entitystockallocation.EntityID(entityID)).
		Exec(ctx); err != nil {
		return err
	}
	created := make([]*ent.EntityStockAllocation, 0, len(allocations))
	for _, allocation := range allocations {
		if allocation.Quantity <= stockEpsilon {
			continue
		}
		if len(created) > 0 {
			rows, err := r.queryRows(ctx, client, entityID, false)
			if err != nil {
				return err
			}
			if err := r.ensureMultiLocationEligible(item, rows, allocation.LocationID); err != nil {
				return err
			}
		}
		row, err := client.EntityStockAllocation.Create().
			SetEntityID(entityID).
			SetNillableLocationID(allocation.LocationID).
			SetQuantity(allocation.Quantity).
			SetIsDefault(allocation.IsDefault).
			Save(ctx)
		if err != nil {
			return err
		}
		created = append(created, row)
	}
	if len(created) > 0 && defaultCount == 0 {
		if err := client.EntityStockAllocation.UpdateOneID(created[0].ID).SetIsDefault(true).Exec(ctx); err != nil {
			return err
		}
	}
	if _, err := r.promoteAndSync(ctx, client, item); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

//nolint:gocyclo // Resolution coordinates every affected allocation in one atomic transaction.
func (r *StockRepository) ResolveLocation(ctx context.Context, gid, actorID, locationID uuid.UUID, request LocationStockResolutionRequest) (LocationStockResolutionResult, error) {
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.IdempotencyKey == "" {
		return LocationStockResolutionResult{}, stockError("idempotency_key_required", "idempotencyKey is required")
	}
	if request.Action == "remove" && !request.Confirmed {
		return LocationStockResolutionResult{}, stockError("confirmation_required", "explicit confirmation is required")
	}
	if request.Action == stockOperationTransfer && request.DestinationLocationID == nil {
		return LocationStockResolutionResult{}, stockError("invalid_resolution", "destinationLocationId is required for transfer")
	}
	if request.Action == "remove" && request.DestinationLocationID != nil {
		return LocationStockResolutionResult{}, stockError("invalid_resolution", "remove must not include destinationLocationId")
	}
	if request.Action != stockOperationTransfer && request.Action != "remove" {
		return LocationStockResolutionResult{}, stockError("invalid_resolution", "action must be transfer or remove")
	}
	if request.DestinationLocationID != nil && *request.DestinationLocationID == locationID {
		return LocationStockResolutionResult{}, stockError("same_location", "destination must differ from the deleted location")
	}
	hashInput := struct {
		LocationID uuid.UUID                      `json:"locationId"`
		Request    LocationStockResolutionRequest `json:"request"`
	}{locationID, request}
	hashBytes, err := json.Marshal(hashInput)
	if err != nil {
		return LocationStockResolutionResult{}, err
	}
	sum := sha256.Sum256(hashBytes)
	requestHash := hex.EncodeToString(sum[:])

	existing, err := r.db.EntityStockTransaction.Query().
		Where(entitystocktransaction.GroupID(gid), entitystocktransaction.IdempotencyKey(request.IdempotencyKey)).
		Only(ctx)
	if err == nil {
		if existing.RequestHash != requestHash {
			return LocationStockResolutionResult{}, stockError("idempotency_conflict", "idempotency key was already used with a different request")
		}
		return r.LocationResolutionState(ctx, gid, locationID)
	}
	if !ent.IsNotFound(err) {
		return LocationStockResolutionResult{}, err
	}

	tx, err := r.db.Tx(ctx)
	if err != nil {
		return LocationStockResolutionResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	client := tx.Client()
	if err := r.validateLocation(ctx, client, gid, &locationID); err != nil {
		return LocationStockResolutionResult{}, err
	}
	if err := r.validateLocation(ctx, client, gid, request.DestinationLocationID); err != nil {
		return LocationStockResolutionResult{}, err
	}
	buildSourceQuery := func() *ent.EntityStockAllocationQuery {
		return client.EntityStockAllocation.Query().
			Where(
				entitystockallocation.LocationID(locationID),
				entitystockallocation.QuantityGT(stockEpsilon),
				entitystockallocation.HasEntityWith(entity.HasGroupWith(group.ID(gid))),
			).
			WithEntity().
			Order(entitystockallocation.ByCreatedAt(), entitystockallocation.ByID())
	}
	sourceRows, err := buildSourceQuery().
		Where(func(s *entsql.Selector) { s.ForUpdate() }).
		All(ctx)
	if err != nil && strings.Contains(err.Error(), "FOR UPDATE/SHARE not supported in SQLite") {
		sourceRows, err = buildSourceQuery().All(ctx)
	}
	if err != nil {
		return LocationStockResolutionResult{}, err
	}
	result := LocationStockResolutionResult{
		LocationID:  locationID,
		ItemCount:   len(sourceRows),
		Allocations: make([]LocationStockConflict, 0, len(sourceRows)),
	}
	for i, source := range sourceRows {
		result.TotalQuantity += source.Quantity
		result.Allocations = append(result.Allocations, LocationStockConflict{
			EntityID: source.EntityID, EntityName: source.Edges.Entity.Name,
			Quantity: source.Quantity, IsDefault: source.IsDefault,
		})
		item, err := client.Entity.Query().
			Where(entity.ID(source.EntityID), entity.HasGroupWith(group.ID(gid))).
			WithParent(func(q *ent.EntityQuery) { q.WithEntityType() }).
			WithChildren().
			Only(ctx)
		if err != nil {
			return LocationStockResolutionResult{}, err
		}
		rows, err := r.queryRows(ctx, client, source.EntityID, true)
		if err != nil {
			return LocationStockResolutionResult{}, err
		}
		beforeTotal := stockStateFromRows(rows).TotalQuantity
		destinationBefore := 0.0
		if destination := findAllocation(rows, request.DestinationLocationID); destination != nil {
			destinationBefore = destination.Quantity
		}
		if err := r.saveAllocation(ctx, client, item, rows, &locationID, 0); err != nil {
			return LocationStockResolutionResult{}, err
		}
		var operation entitystocktransaction.Operation
		var destinationAfter *float64
		if request.Action == stockOperationTransfer {
			operation = entitystocktransaction.OperationResolveTransfer
			rows, err = r.queryRows(ctx, client, source.EntityID, false)
			if err != nil {
				return LocationStockResolutionResult{}, err
			}
			after := destinationBefore + source.Quantity
			if err := r.saveAllocation(ctx, client, item, rows, request.DestinationLocationID, after); err != nil {
				return LocationStockResolutionResult{}, err
			}
			destinationAfter = &after
		} else {
			operation = entitystocktransaction.OperationResolveRemove
		}
		state, err := r.promoteAndSync(ctx, client, item)
		if err != nil {
			return LocationStockResolutionResult{}, err
		}
		key := request.IdempotencyKey
		if i > 0 {
			key += ":" + source.EntityID.String()
		}
		builder := client.EntityStockTransaction.Create().
			SetGroupID(gid).
			SetEntityID(source.EntityID).
			SetOperation(operation).
			SetWorkflow(request.Workflow).
			SetSourceLocationID(locationID).
			SetNillableDestinationLocationID(request.DestinationLocationID).
			SetQuantity(source.Quantity).
			SetBeforeTotal(beforeTotal).
			SetAfterTotal(state.TotalQuantity).
			SetSourceBefore(source.Quantity).
			SetSourceAfter(0).
			SetDestinationBefore(destinationBefore).
			SetNillableDestinationAfter(destinationAfter).
			SetReason(request.Reason).
			SetIdempotencyKey(key).
			SetRequestHash(requestHash)
		if actorID != uuid.Nil {
			builder.SetActorID(actorID)
		}
		if err := builder.Exec(ctx); err != nil {
			return LocationStockResolutionResult{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return LocationStockResolutionResult{}, err
	}
	committed = true
	if r.bus != nil {
		r.bus.Publish(eventbus.EventEntityMutation, eventbus.GroupMutationEvent{GID: gid})
	}
	return result, nil
}

func (r *StockRepository) LocationHasStock(ctx context.Context, gid, locationID uuid.UUID) (bool, error) {
	return r.db.EntityStockAllocation.Query().
		Where(
			entitystockallocation.LocationID(locationID),
			entitystockallocation.QuantityGT(stockEpsilon),
			entitystockallocation.HasEntityWith(entity.HasGroupWith(group.ID(gid))),
		).
		Exist(ctx)
}
