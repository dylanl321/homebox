package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
	"github.com/sysadminsmedia/homebox/backend/internal/web/adapters"
)

func locationLayoutError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repo.ErrLocationLayoutConflict):
		return validate.NewRequestError(err, http.StatusConflict)
	case errors.Is(err, repo.ErrLocationLayoutGeometry), errors.Is(err, repo.ErrLocationLayoutTarget):
		return validate.NewRequestError(err, http.StatusBadRequest)
	case errors.Is(err, repo.ErrLocationLayoutOwner), ent.IsNotFound(err):
		return validate.NewRequestError(err, http.StatusNotFound)
	default:
		return err
	}
}

// HandleLocationLayoutGet godoc
//
//	@Summary	Get a location overhead layout
//	@Tags		Locations
//	@Produce	json
//	@Param		id	path		string	true	"Location ID"
//	@Success	200	{object}	repo.LocationLayoutOut
//	@Router		/v1/entities/{id}/layout [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleLocationLayoutGet() errchain.HandlerFunc {
	fn := func(r *http.Request, id uuid.UUID) (repo.LocationLayoutOut, error) {
		auth := services.NewContext(r.Context())
		out, err := ctrl.repo.LocationLayouts.Get(auth, auth.GID, id)
		return out, locationLayoutError(err)
	}
	return adapters.CommandID("id", fn, http.StatusOK)
}

// HandleLocationLayoutReplace godoc
//
//	@Summary	Replace a location overhead layout
//	@Tags		Locations
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"Location ID"
//	@Param		payload	body		repo.LocationLayoutReplace	true	"Complete layout"
//	@Success	200		{object}	repo.LocationLayoutOut
//	@Failure	409		{object}	validate.ErrorResponse
//	@Router		/v1/entities/{id}/layout [PUT]
//	@Security	Bearer
func (ctrl *V1Controller) HandleLocationLayoutReplace() errchain.HandlerFunc {
	fn := func(r *http.Request, id uuid.UUID, input repo.LocationLayoutReplace) (repo.LocationLayoutOut, error) {
		auth := services.NewContext(r.Context())
		out, err := ctrl.repo.LocationLayouts.Replace(auth, auth.GID, id, input)
		return out, locationLayoutError(err)
	}
	return adapters.ActionID("id", fn, http.StatusOK)
}

// HandleLocationLayoutDelete godoc
//
//	@Summary	Delete a location overhead layout
//	@Tags		Locations
//	@Param		id	path	string	true	"Location ID"
//	@Success	204
//	@Router		/v1/entities/{id}/layout [DELETE]
//	@Security	Bearer
func (ctrl *V1Controller) HandleLocationLayoutDelete() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := adapters.RouteUUID(r, "id")
		if err != nil {
			return err
		}
		auth := services.NewContext(r.Context())
		if err := ctrl.repo.LocationLayouts.Delete(auth, auth.GID, id); err != nil {
			return locationLayoutError(err)
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}
