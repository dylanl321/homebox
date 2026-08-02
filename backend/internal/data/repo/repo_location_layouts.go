package repo

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entity"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/entitytype"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/group"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/locationlayout"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/locationlayoutelement"
)

const (
	LocationLayoutCanvasWidth  = 1000
	LocationLayoutCanvasHeight = 700
)

var (
	ErrLocationLayoutConflict = errors.New("location layout revision conflict")
	ErrLocationLayoutGeometry = errors.New("invalid location layout geometry")
	ErrLocationLayoutTarget   = errors.New("layout target must be a direct child location")
	ErrLocationLayoutOwner    = errors.New("layout owner must be a location in this collection")
)

type LocationLayoutWall struct {
	ID     uuid.UUID `json:"id"`
	X      float64   `json:"x"`
	Y      float64   `json:"y"`
	EndX   float64   `json:"endX"`
	EndY   float64   `json:"endY"`
	ZOrder int       `json:"zOrder"`
}

type LocationLayoutPlacement struct {
	ID        uuid.UUID `json:"id"`
	TargetID  uuid.UUID `json:"targetId"`
	Name      string    `json:"name"`
	ItemCount int       `json:"itemCount"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Width     float64   `json:"width"`
	Height    float64   `json:"height"`
	Rotation  float64   `json:"rotation"`
	ZOrder    int       `json:"zOrder"`
}

type LocationLayoutOut struct {
	CanvasWidth  int                       `json:"canvasWidth"`
	CanvasHeight int                       `json:"canvasHeight"`
	Revision     int                       `json:"revision"`
	Walls        []LocationLayoutWall      `json:"walls"`
	Locations    []LocationLayoutPlacement `json:"locations"`
}

type LocationLayoutElementInput struct {
	ID       uuid.UUID `json:"id"`
	Kind     string    `json:"kind"`
	TargetID uuid.UUID `json:"targetId,omitempty"`
	X        float64   `json:"x"`
	Y        float64   `json:"y"`
	Width    float64   `json:"width,omitempty"`
	Height   float64   `json:"height,omitempty"`
	EndX     float64   `json:"endX,omitempty"`
	EndY     float64   `json:"endY,omitempty"`
	Rotation float64   `json:"rotation,omitempty"`
	ZOrder   int       `json:"zOrder"`
}

type LocationLayoutReplace struct {
	ExpectedRevision int                          `json:"expectedRevision"`
	Elements         []LocationLayoutElementInput `json:"elements"`
}

type LocationLayoutRepository struct {
	db *ent.Client
}

func NewLocationLayoutRepository(db *ent.Client) *LocationLayoutRepository {
	return &LocationLayoutRepository{db: db}
}

func emptyLocationLayout() LocationLayoutOut {
	return LocationLayoutOut{
		CanvasWidth:  LocationLayoutCanvasWidth,
		CanvasHeight: LocationLayoutCanvasHeight,
		Walls:        []LocationLayoutWall{},
		Locations:    []LocationLayoutPlacement{},
	}
}

func (r *LocationLayoutRepository) ownerExists(ctx context.Context, db *ent.Client, gid, ownerID uuid.UUID) (bool, error) {
	return db.Entity.Query().
		Where(
			entity.ID(ownerID),
			entity.HasGroupWith(group.ID(gid)),
			entity.HasEntityTypeWith(entitytype.IsLocation(true)),
		).
		Exist(ctx)
}

func (r *LocationLayoutRepository) Get(ctx context.Context, gid, ownerID uuid.UUID) (LocationLayoutOut, error) {
	out := emptyLocationLayout()

	ok, err := r.ownerExists(ctx, r.db, gid, ownerID)
	if err != nil {
		return out, err
	}
	if !ok {
		return out, ErrLocationLayoutOwner
	}

	layout, err := r.db.LocationLayout.Query().
		Where(locationlayout.HasOwnerWith(entity.ID(ownerID), entity.HasGroupWith(group.ID(gid)))).
		Only(ctx)
	if ent.IsNotFound(err) {
		return out, nil
	}
	if err != nil {
		return out, err
	}

	elements, err := layout.QueryElements().
		WithTarget().
		Order(ent.Asc(locationlayoutelement.FieldZOrder), ent.Asc(locationlayoutelement.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return out, err
	}

	out.Revision = layout.Revision
	for _, element := range elements {
		switch element.Kind {
		case locationlayoutelement.KindWall:
			out.Walls = append(out.Walls, LocationLayoutWall{
				ID: element.ID, X: element.X, Y: element.Y, EndX: element.EndX, EndY: element.EndY, ZOrder: element.ZOrder,
			})
		case locationlayoutelement.KindLocation:
			target := element.Edges.Target
			if target == nil {
				continue
			}
			valid, err := target.QueryParent().
				Where(entity.ID(ownerID), entity.HasGroupWith(group.ID(gid))).
				Exist(ctx)
			if err != nil {
				return out, err
			}
			if !valid {
				continue
			}
			isLocation, err := target.QueryEntityType().Where(entitytype.IsLocation(true)).Exist(ctx)
			if err != nil {
				return out, err
			}
			if !isLocation {
				continue
			}
			itemCount, err := target.QueryChildren().
				Where(entity.HasEntityTypeWith(entitytype.IsLocation(false)), entity.Archived(false)).
				Count(ctx)
			if err != nil {
				return out, err
			}
			out.Locations = append(out.Locations, LocationLayoutPlacement{
				ID: element.ID, TargetID: target.ID, Name: target.Name, ItemCount: itemCount,
				X: element.X, Y: element.Y, Width: element.Width, Height: element.Height,
				Rotation: element.Rotation, ZOrder: element.ZOrder,
			})
		}
	}
	return out, nil
}

func normalizeRotation(rotation float64) float64 {
	rotation = math.Mod(rotation+180, 360)
	if rotation < 0 {
		rotation += 360
	}
	return rotation - 180
}

func finite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func validateLocationLayoutElements(elements []LocationLayoutElementInput) error {
	targets := make(map[uuid.UUID]struct{})
	for i := range elements {
		element := &elements[i]
		if !finite(element.X, element.Y, element.Width, element.Height, element.EndX, element.EndY, element.Rotation) {
			return fmt.Errorf("%w: element %d contains a non-finite number", ErrLocationLayoutGeometry, i)
		}
		if element.X < 0 || element.X > 1 || element.Y < 0 || element.Y > 1 {
			return fmt.Errorf("%w: element %d origin is outside the canvas", ErrLocationLayoutGeometry, i)
		}
		switch element.Kind {
		case locationlayoutelement.KindWall.String():
			if element.TargetID != uuid.Nil || element.EndX < 0 || element.EndX > 1 || element.EndY < 0 || element.EndY > 1 {
				return fmt.Errorf("%w: wall %d is invalid", ErrLocationLayoutGeometry, i)
			}
		case locationlayoutelement.KindLocation.String():
			if element.TargetID == uuid.Nil || element.Width <= 0 || element.Height <= 0 ||
				element.X+element.Width > 1 || element.Y+element.Height > 1 {
				return fmt.Errorf("%w: location %d bounds are invalid", ErrLocationLayoutGeometry, i)
			}
			if _, exists := targets[element.TargetID]; exists {
				return fmt.Errorf("%w: target %s appears more than once", ErrLocationLayoutTarget, element.TargetID)
			}
			targets[element.TargetID] = struct{}{}
			element.Rotation = normalizeRotation(element.Rotation)
		default:
			return fmt.Errorf("%w: unknown element kind %q", ErrLocationLayoutGeometry, element.Kind)
		}
	}
	return nil
}

func (r *LocationLayoutRepository) Replace(
	ctx context.Context,
	gid, ownerID uuid.UUID,
	input LocationLayoutReplace,
) (LocationLayoutOut, error) {
	out := emptyLocationLayout()
	if input.ExpectedRevision < 0 {
		return out, ErrLocationLayoutConflict
	}
	if err := validateLocationLayoutElements(input.Elements); err != nil {
		return out, err
	}

	tx, err := r.db.Tx(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback() }()

	ownerOK, err := tx.Entity.Query().
		Where(entity.ID(ownerID), entity.HasGroupWith(group.ID(gid)), entity.HasEntityTypeWith(entitytype.IsLocation(true))).
		Exist(ctx)
	if err != nil {
		return out, err
	}
	if !ownerOK {
		return out, ErrLocationLayoutOwner
	}

	targetIDs := make([]uuid.UUID, 0)
	for _, element := range input.Elements {
		if element.Kind == locationlayoutelement.KindLocation.String() {
			targetIDs = append(targetIDs, element.TargetID)
		}
	}
	if len(targetIDs) > 0 {
		count, err := tx.Entity.Query().
			Where(
				entity.IDIn(targetIDs...),
				entity.HasGroupWith(group.ID(gid)),
				entity.HasParentWith(entity.ID(ownerID)),
				entity.HasEntityTypeWith(entitytype.IsLocation(true)),
			).
			Count(ctx)
		if err != nil {
			return out, err
		}
		if count != len(targetIDs) {
			return out, ErrLocationLayoutTarget
		}
	}

	layout, err := tx.LocationLayout.Query().
		Where(locationlayout.HasOwnerWith(entity.ID(ownerID), entity.HasGroupWith(group.ID(gid)))).
		Only(ctx)
	switch {
	case ent.IsNotFound(err):
		if input.ExpectedRevision != 0 {
			return out, ErrLocationLayoutConflict
		}
		layout, err = tx.LocationLayout.Create().
			SetOwnerID(ownerID).
			SetCanvasWidth(LocationLayoutCanvasWidth).
			SetCanvasHeight(LocationLayoutCanvasHeight).
			SetRevision(1).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				return out, ErrLocationLayoutConflict
			}
			return out, err
		}
	case err != nil:
		return out, err
	default:
		if layout.Revision != input.ExpectedRevision {
			return out, ErrLocationLayoutConflict
		}
		updated, err := tx.LocationLayout.Update().
			Where(locationlayout.ID(layout.ID), locationlayout.Revision(input.ExpectedRevision)).
			AddRevision(1).
			Save(ctx)
		if err != nil {
			return out, err
		}
		if updated != 1 {
			return out, ErrLocationLayoutConflict
		}
		layout.Revision++
		if _, err := tx.LocationLayoutElement.Delete().
			Where(locationlayoutelement.HasLayoutWith(locationlayout.ID(layout.ID))).
			Exec(ctx); err != nil {
			return out, err
		}
	}

	for _, element := range input.Elements {
		create := tx.LocationLayoutElement.Create().
			SetLayoutID(layout.ID).
			SetKind(locationlayoutelement.Kind(element.Kind)).
			SetX(element.X).
			SetY(element.Y).
			SetWidth(element.Width).
			SetHeight(element.Height).
			SetEndX(element.EndX).
			SetEndY(element.EndY).
			SetRotation(element.Rotation).
			SetZOrder(element.ZOrder)
		if element.ID != uuid.Nil {
			create.SetID(element.ID)
		}
		if element.TargetID != uuid.Nil {
			create.SetTargetID(element.TargetID)
		}
		if _, err := create.Save(ctx); err != nil {
			return out, err
		}
	}

	if err := tx.Commit(); err != nil {
		if ent.IsConstraintError(err) {
			return out, ErrLocationLayoutConflict
		}
		return out, err
	}
	return r.Get(ctx, gid, ownerID)
}

func (r *LocationLayoutRepository) Delete(ctx context.Context, gid, ownerID uuid.UUID) error {
	ok, err := r.ownerExists(ctx, r.db, gid, ownerID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLocationLayoutOwner
	}
	_, err = r.db.LocationLayout.Delete().
		Where(locationlayout.HasOwnerWith(entity.ID(ownerID), entity.HasGroupWith(group.ID(gid)))).
		Exec(ctx)
	return err
}
