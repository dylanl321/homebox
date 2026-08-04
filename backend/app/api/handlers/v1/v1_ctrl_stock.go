package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
	"github.com/sysadminsmedia/homebox/backend/internal/web/adapters"
)

func stockRequestError(err error) error {
	if err == nil {
		return nil
	}
	if ent.IsNotFound(err) {
		return validate.NewRequestError(err, http.StatusNotFound)
	}
	var stockErr *repo.StockError
	if !errors.As(err, &stockErr) {
		return err
	}
	switch stockErr.Code {
	case "idempotency_conflict", "insufficient_stock", "multi_location_stock",
		"container_item", "nested_item", "location_has_stock", "allocation_not_found":
		return validate.NewRequestError(err, http.StatusConflict)
	default:
		return validate.NewRequestError(err, http.StatusBadRequest)
	}
}

// HandleEntityStockGet godoc
//
//	@Summary	Get item stock allocations
//	@Tags		Stock
//	@Produce	json
//	@Param		id	path		string	true	"Entity ID"
//	@Success	200	{object}	repo.StockState
//	@Router		/v1/entities/{id}/stock [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleEntityStockGet() errchain.HandlerFunc {
	fn := func(r *http.Request, id uuid.UUID) (repo.StockState, error) {
		ctx := services.NewContext(r.Context())
		state, err := ctrl.repo.Stock.Get(ctx, ctx.GID, id)
		return state, stockRequestError(err)
	}
	return adapters.CommandID("id", fn, http.StatusOK)
}

// HandleEntityStockPost godoc
//
//	@Summary	Adjust, set, or transfer item stock
//	@Tags		Stock
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"Entity ID"
//	@Param		payload	body		repo.StockOperationRequest	true	"Stock operation"
//	@Success	200		{object}	repo.StockState
//	@Router		/v1/entities/{id}/stock [POST]
//	@Security	Bearer
func (ctrl *V1Controller) HandleEntityStockPost() errchain.HandlerFunc {
	fn := func(r *http.Request, id uuid.UUID, body repo.StockOperationRequest) (repo.StockState, error) {
		ctx := services.NewContext(r.Context())
		state, _, err := ctrl.repo.Stock.Operate(ctx, ctx.GID, ctx.UID, id, body)
		return state, stockRequestError(err)
	}
	return adapters.ActionID("id", fn, http.StatusOK)
}

// HandleEntityStockDefaultPost godoc
//
//	@Summary	Set the compatibility default stock location
//	@Tags		Stock
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"Entity ID"
//	@Param		payload	body		repo.SetDefaultStockRequest	true	"Default allocation"
//	@Success	200		{object}	repo.StockState
//	@Router		/v1/entities/{id}/stock/default [POST]
//	@Security	Bearer
func (ctrl *V1Controller) HandleEntityStockDefaultPost() errchain.HandlerFunc {
	fn := func(r *http.Request, id uuid.UUID, body repo.SetDefaultStockRequest) (repo.StockState, error) {
		ctx := services.NewContext(r.Context())
		state, err := ctrl.repo.Stock.SetDefault(ctx, ctx.GID, id, body.LocationID)
		return state, stockRequestError(err)
	}
	return adapters.ActionID("id", fn, http.StatusOK)
}

// HandleStockTransactionsGet godoc
//
//	@Summary	Get stock transaction history
//	@Tags		Stock
//	@Produce	json
//	@Param		entityId	query		string	false	"Entity ID"
//	@Param		locationId	query		string	false	"Location ID"
//	@Param		page		query		int		false	"Page"
//	@Param		pageSize	query		int		false	"Page size"
//	@Success	200			{object}	repo.PaginationResult[repo.StockTransaction]
//	@Router		/v1/stock-transactions [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleStockTransactionsGet() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := services.NewContext(r.Context())
		query := repo.StockTransactionQuery{
			Page:     queryIntOrNegativeOne(r.URL.Query().Get("page")),
			PageSize: queryIntOrNegativeOne(r.URL.Query().Get("pageSize")),
		}
		if raw := r.URL.Query().Get("entityId"); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return validate.NewRequestError(err, http.StatusBadRequest)
			}
			query.EntityID = &id
		}
		if raw := r.URL.Query().Get("locationId"); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return validate.NewRequestError(err, http.StatusBadRequest)
			}
			query.LocationID = &id
		}
		out, err := ctrl.repo.Stock.Transactions(ctx, ctx.GID, query)
		if err != nil {
			return stockRequestError(err)
		}
		return server.JSON(w, http.StatusOK, out)
	}
}

// HandleLocationStockResolutionGet godoc
//
//	@Summary	List stock blocking location deletion
//	@Tags		Stock
//	@Produce	json
//	@Param		locationId	path		string	true	"Location ID"
//	@Success	200			{object}	repo.LocationStockResolutionResult
//	@Router		/v1/entities/{locationId}/stock-resolution [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleLocationStockResolutionGet() errchain.HandlerFunc {
	fn := func(r *http.Request, id uuid.UUID) (repo.LocationStockResolutionResult, error) {
		ctx := services.NewContext(r.Context())
		out, err := ctrl.repo.Stock.LocationResolutionState(ctx, ctx.GID, id)
		return out, stockRequestError(err)
	}
	return adapters.CommandID("locationId", fn, http.StatusOK)
}

// HandleLocationStockResolutionPost godoc
//
//	@Summary	Resolve stock blocking location deletion
//	@Tags		Stock
//	@Accept		json
//	@Produce	json
//	@Param		locationId	path		string								true	"Location ID"
//	@Param		payload		body		repo.LocationStockResolutionRequest	true	"Resolution"
//	@Success	200			{object}	repo.LocationStockResolutionResult
//	@Router		/v1/entities/{locationId}/stock-resolution [POST]
//	@Security	Bearer
func (ctrl *V1Controller) HandleLocationStockResolutionPost() errchain.HandlerFunc {
	fn := func(r *http.Request, id uuid.UUID, body repo.LocationStockResolutionRequest) (repo.LocationStockResolutionResult, error) {
		ctx := services.NewContext(r.Context())
		out, err := ctrl.repo.Stock.ResolveLocation(ctx, ctx.GID, ctx.UID, id, body)
		return out, stockRequestError(err)
	}
	return adapters.ActionID("locationId", fn, http.StatusOK)
}
