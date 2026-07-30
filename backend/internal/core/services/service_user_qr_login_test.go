package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/pkgs/hasher"
)

func TestCreateQRLoginToken_HappyPath(t *testing.T) {
	ctx := context.Background()
	usr := newTestUserWithPassword(t, "qr-login-password")

	detail, err := tSvc.User.CreateQRLoginToken(ctx, usr.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, detail.Token)
	assert.True(t, detail.ExpiresAt.After(time.Now()))
	assert.True(t, detail.ExpiresAt.Before(time.Now().Add(3*time.Minute)))

	hash := hasher.HashToken(detail.Token)
	got, err := tRepos.QRLoginTokens.GetValidByHash(ctx, hash)
	require.NoError(t, err)
	assert.Equal(t, usr.ID, got.UserID)
}

func TestCreateQRLoginToken_InvalidatesPrevious(t *testing.T) {
	ctx := context.Background()
	usr := newTestUserWithPassword(t, "qr-login-password")

	first, err := tSvc.User.CreateQRLoginToken(ctx, usr.ID)
	require.NoError(t, err)

	second, err := tSvc.User.CreateQRLoginToken(ctx, usr.ID)
	require.NoError(t, err)
	assert.NotEqual(t, first.Token, second.Token)

	_, err = tRepos.QRLoginTokens.GetValidByHash(ctx, hasher.HashToken(first.Token))
	require.Error(t, err)
	assert.True(t, ent.IsNotFound(err), "first token should be invalidated")

	got, err := tRepos.QRLoginTokens.GetValidByHash(ctx, hasher.HashToken(second.Token))
	require.NoError(t, err)
	assert.Equal(t, usr.ID, got.UserID)
}

func TestExchangeQRLoginToken_HappyPath(t *testing.T) {
	ctx := context.Background()
	usr := newTestUserWithPassword(t, "qr-login-password")

	detail, err := tSvc.User.CreateQRLoginToken(ctx, usr.ID)
	require.NoError(t, err)

	session, err := tSvc.User.ExchangeQRLoginToken(ctx, detail.Token, true)
	require.NoError(t, err)
	assert.NotEmpty(t, session.Raw)
	assert.NotEmpty(t, session.AttachmentToken)

	// Token is single-use.
	_, err = tSvc.User.ExchangeQRLoginToken(ctx, detail.Token, false)
	require.ErrorIs(t, err, ErrorQRLoginInvalid)
}

func TestExchangeQRLoginToken_InvalidToken(t *testing.T) {
	_, err := tSvc.User.ExchangeQRLoginToken(context.Background(), "not-a-real-token", false)
	require.ErrorIs(t, err, ErrorQRLoginInvalid)
}

func TestExchangeQRLoginToken_EmptyToken(t *testing.T) {
	_, err := tSvc.User.ExchangeQRLoginToken(context.Background(), "", false)
	require.ErrorIs(t, err, ErrorQRLoginInvalid)
}

func TestExchangeQRLoginToken_Expired(t *testing.T) {
	ctx := context.Background()
	usr := newTestUserWithPassword(t, "qr-login-password")

	tok := hasher.GenerateToken()
	_, err := tRepos.QRLoginTokens.Create(ctx, usr.ID, tok.Hash, time.Now().Add(-time.Minute))
	require.NoError(t, err)

	_, err = tSvc.User.ExchangeQRLoginToken(ctx, tok.Raw, false)
	require.ErrorIs(t, err, ErrorQRLoginInvalid)
}
