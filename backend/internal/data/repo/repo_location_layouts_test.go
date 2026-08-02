package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/locationlayoutelement"
)

func createTestLocation(t *testing.T, gid uuid.UUID, name string, parentID uuid.UUID) EntityOut {
	t.Helper()
	locationType, err := tRepos.EntityTypes.GetDefault(context.Background(), gid, true)
	require.NoError(t, err)
	out, err := tRepos.Entities.Create(context.Background(), gid, EntityCreate{
		Name:         name,
		ParentID:     parentID,
		EntityTypeID: locationType.ID,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tRepos.Entities.Delete(context.Background(), out.ID) })
	return out
}

func validLayoutInput(targetID uuid.UUID, revision int) LocationLayoutReplace {
	return LocationLayoutReplace{
		ExpectedRevision: revision,
		Elements: []LocationLayoutElementInput{
			{Kind: "wall", X: 0.1, Y: 0.1, EndX: 0.9, EndY: 0.1},
			{Kind: "location", TargetID: targetID, X: 0.2, Y: 0.2, Width: 0.2, Height: 0.15, Rotation: 270},
		},
	}
}

func TestLocationLayoutReplaceAndGet(t *testing.T) {
	owner := createTestLocation(t, tGroup.ID, "Room", uuid.Nil)
	child := createTestLocation(t, tGroup.ID, "Shelf", owner.ID)

	out, err := tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, validLayoutInput(child.ID, 0))
	require.NoError(t, err)
	assert.Equal(t, 1, out.Revision)
	require.Len(t, out.Walls, 1)
	require.Len(t, out.Locations, 1)
	assert.InDelta(t, -90.0, out.Locations[0].Rotation, 0.000001)
	assert.Equal(t, "Shelf", out.Locations[0].Name)

	loaded, err := tRepos.LocationLayouts.Get(context.Background(), tGroup.ID, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, out, loaded)
}

func TestLocationLayoutTenantIsolation(t *testing.T) {
	owner := createTestLocation(t, tGroup.ID, "Private room", uuid.Nil)
	otherGroup, err := tRepos.Groups.GroupCreate(context.Background(), "layout-other", uuid.Nil)
	require.NoError(t, err)

	_, err = tRepos.LocationLayouts.Get(context.Background(), otherGroup.ID, owner.ID)
	require.ErrorIs(t, err, ErrLocationLayoutOwner)
	_, err = tRepos.LocationLayouts.Replace(context.Background(), otherGroup.ID, owner.ID, LocationLayoutReplace{})
	require.ErrorIs(t, err, ErrLocationLayoutOwner)
}

func TestLocationLayoutRejectsInvalidTargetsAndGeometry(t *testing.T) {
	owner := createTestLocation(t, tGroup.ID, "Room", uuid.Nil)
	notChild := createTestLocation(t, tGroup.ID, "Other shelf", uuid.Nil)

	_, err := tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, validLayoutInput(notChild.ID, 0))
	require.ErrorIs(t, err, ErrLocationLayoutTarget)

	child := createTestLocation(t, tGroup.ID, "Shelf", owner.ID)
	invalid := validLayoutInput(child.ID, 0)
	invalid.Elements[1].Width = 0.9
	_, err = tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, invalid)
	assert.ErrorIs(t, err, ErrLocationLayoutGeometry)
}

func TestLocationLayoutRejectsDuplicateTargetsAndRevisionConflicts(t *testing.T) {
	owner := createTestLocation(t, tGroup.ID, "Room", uuid.Nil)
	child := createTestLocation(t, tGroup.ID, "Shelf", owner.ID)

	duplicate := validLayoutInput(child.ID, 0)
	duplicate.Elements = append(duplicate.Elements, duplicate.Elements[1])
	_, err := tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, duplicate)
	require.ErrorIs(t, err, ErrLocationLayoutTarget)

	_, err = tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, validLayoutInput(child.ID, 0))
	require.NoError(t, err)
	_, err = tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, validLayoutInput(child.ID, 0))
	assert.ErrorIs(t, err, ErrLocationLayoutConflict)
}

func TestLocationLayoutTargetDeletionCascadesPlacement(t *testing.T) {
	owner := createTestLocation(t, tGroup.ID, "Room", uuid.Nil)
	child := createTestLocation(t, tGroup.ID, "Shelf", owner.ID)
	_, err := tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, validLayoutInput(child.ID, 0))
	require.NoError(t, err)

	require.NoError(t, tRepos.Entities.Delete(context.Background(), child.ID))
	count, err := tClient.LocationLayoutElement.Query().
		Where(locationlayoutelement.HasTarget()).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)

	out, err := tRepos.LocationLayouts.Get(context.Background(), tGroup.ID, owner.ID)
	require.NoError(t, err)
	assert.Empty(t, out.Locations)
	assert.Len(t, out.Walls, 1)
}

func TestLocationLayoutMovedTargetIsOmittedAndPruned(t *testing.T) {
	owner := createTestLocation(t, tGroup.ID, "Room", uuid.Nil)
	other := createTestLocation(t, tGroup.ID, "Other room", uuid.Nil)
	child := createTestLocation(t, tGroup.ID, "Shelf", owner.ID)
	created, err := tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, validLayoutInput(child.ID, 0))
	require.NoError(t, err)

	_, err = tRepos.Entities.UpdateByGroup(context.Background(), tGroup.ID, EntityUpdate{
		ID: child.ID, Name: child.Name, ParentID: other.ID, EntityTypeID: child.EntityType.ID,
	})
	require.NoError(t, err)

	out, err := tRepos.LocationLayouts.Get(context.Background(), tGroup.ID, owner.ID)
	require.NoError(t, err)
	assert.Empty(t, out.Locations)

	saved, err := tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, LocationLayoutReplace{
		ExpectedRevision: created.Revision,
		Elements: []LocationLayoutElementInput{
			{Kind: "wall", X: 0.1, Y: 0.1, EndX: 0.9, EndY: 0.1},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, saved.Revision)
	assert.Empty(t, saved.Locations)

	count, err := tClient.LocationLayoutElement.Query().
		Where(locationlayoutelement.HasTargetWith()).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestLocationLayoutDelete(t *testing.T) {
	owner := createTestLocation(t, tGroup.ID, "Room", uuid.Nil)
	child := createTestLocation(t, tGroup.ID, "Shelf", owner.ID)
	_, err := tRepos.LocationLayouts.Replace(context.Background(), tGroup.ID, owner.ID, validLayoutInput(child.ID, 0))
	require.NoError(t, err)
	require.NoError(t, tRepos.LocationLayouts.Delete(context.Background(), tGroup.ID, owner.ID))

	out, err := tRepos.LocationLayouts.Get(context.Background(), tGroup.ID, owner.ID)
	require.NoError(t, err)
	assert.Zero(t, out.Revision)
	assert.Empty(t, out.Walls)
	assert.Empty(t, out.Locations)
	require.NotErrorIs(t, err, ErrLocationLayoutConflict)
}
