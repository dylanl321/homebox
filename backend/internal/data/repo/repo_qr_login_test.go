package repo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/qrlogintokens"
)

func TestQRLoginTokens_MarkUsed_RejectsExpired(t *testing.T) {
	ctx := context.Background()

	tok, err := tRepos.QRLoginTokens.Create(ctx, tUser.ID, []byte("qr-hash-expired"), time.Now().Add(-time.Minute))
	require.NoError(t, err)

	err = tRepos.QRLoginTokens.MarkUsed(ctx, tok.ID, time.Now())
	require.ErrorIs(t, err, ErrQRLoginTokenAlreadyClaimed)

	row, err := tClient.QRLoginTokens.Query().
		Where(qrlogintokens.ID(tok.ID)).
		Only(ctx)
	require.NoError(t, err)
	assert.Nil(t, row.UsedAt)
}

func TestQRLoginTokens_MarkUsed_SingleUse(t *testing.T) {
	ctx := context.Background()

	tok, err := tRepos.QRLoginTokens.Create(ctx, tUser.ID, []byte("qr-hash-single-use"), time.Now().Add(time.Minute))
	require.NoError(t, err)

	require.NoError(t, tRepos.QRLoginTokens.MarkUsed(ctx, tok.ID, time.Now()))
	err = tRepos.QRLoginTokens.MarkUsed(ctx, tok.ID, time.Now())
	require.ErrorIs(t, err, ErrQRLoginTokenAlreadyClaimed)
}

func TestQRLoginTokens_GetValidByHash_NotFoundWhenUsed(t *testing.T) {
	ctx := context.Background()
	hash := []byte("qr-hash-used-lookup")

	tok, err := tRepos.QRLoginTokens.Create(ctx, tUser.ID, hash, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.NoError(t, tRepos.QRLoginTokens.MarkUsed(ctx, tok.ID, time.Now()))

	_, err = tRepos.QRLoginTokens.GetValidByHash(ctx, hash)
	require.Error(t, err)
	assert.True(t, ent.IsNotFound(err))
}

func TestQRLoginTokens_PurgeExpired(t *testing.T) {
	ctx := context.Background()

	_, err := tRepos.QRLoginTokens.Create(ctx, tUser.ID, []byte("qr-purge-expired"), time.Now().Add(-time.Hour))
	require.NoError(t, err)

	live, err := tRepos.QRLoginTokens.Create(ctx, tUser.ID, []byte("qr-purge-live"), time.Now().Add(time.Hour))
	require.NoError(t, err)

	deleted, err := tRepos.QRLoginTokens.PurgeExpired(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1)

	_, err = tRepos.QRLoginTokens.GetValidByHash(ctx, []byte("qr-purge-live"))
	require.NoError(t, err)
	assert.Equal(t, live.UserID, tUser.ID)
}
