package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/pkgs/hasher"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	qrLoginTokenTTL     = 2 * time.Minute
	ErrorQRLoginInvalid = errors.New("qr login code is invalid or has expired")
)

// QRLoginTokenDetail is returned when minting a QR login token. The raw token
// is shown once (encoded into the QR); only the hash is stored.
type QRLoginTokenDetail struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// CreateQRLoginToken mints a short-lived single-use token for the given user.
// Any previously unused tokens for that user are invalidated first so only one
// QR login code is live at a time.
func (svc *UserService) CreateQRLoginToken(ctx context.Context, userID uuid.UUID) (QRLoginTokenDetail, error) {
	ctx, span := entityServiceTracer().Start(ctx, "service.UserService.CreateQRLoginToken",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	if _, err := svc.repos.QRLoginTokens.InvalidateUnusedByUser(ctx, userID); err != nil {
		recordServiceSpanError(span, err)
		span.SetAttributes(attribute.String("qr_login.outcome", "invalidate_failed"))
		return QRLoginTokenDetail{}, err
	}

	tok := hasher.GenerateTokenCtx(ctx)
	expiresAt := time.Now().Add(qrLoginTokenTTL)

	created, err := svc.repos.QRLoginTokens.Create(ctx, userID, tok.Hash, expiresAt)
	if err != nil {
		recordServiceSpanError(span, err)
		span.SetAttributes(attribute.String("qr_login.outcome", "create_failed"))
		return QRLoginTokenDetail{}, err
	}

	span.SetAttributes(
		attribute.String("qr_login.outcome", "created"),
		attribute.String("token.expires_at", created.ExpiresAt.Format(time.RFC3339)),
	)
	return QRLoginTokenDetail{
		Token:     tok.Raw,
		ExpiresAt: created.ExpiresAt,
	}, nil
}

// ExchangeQRLoginToken consumes a QR login token and issues a full session for
// the owning user. Invalid, expired, and already-used tokens all return
// ErrorQRLoginInvalid.
func (svc *UserService) ExchangeQRLoginToken(ctx context.Context, rawToken string, extendedSession bool) (UserAuthTokenDetail, error) {
	ctx, span := entityServiceTracer().Start(ctx, "service.UserService.ExchangeQRLoginToken",
		trace.WithAttributes(
			attribute.Int("token.length", len(rawToken)),
			attribute.Bool("session.extended", extendedSession),
		))
	defer span.End()

	if rawToken == "" {
		span.SetAttributes(attribute.String("qr_login.outcome", "missing_token"))
		return UserAuthTokenDetail{}, ErrorQRLoginInvalid
	}

	hash := hasher.HashToken(rawToken)
	tok, err := svc.repos.QRLoginTokens.GetValidByHash(ctx, hash)
	if err != nil {
		if ent.IsNotFound(err) {
			span.SetAttributes(attribute.String("qr_login.outcome", "token_invalid"))
			return UserAuthTokenDetail{}, ErrorQRLoginInvalid
		}
		recordServiceSpanError(span, err)
		span.SetAttributes(attribute.String("qr_login.outcome", "lookup_failed"))
		return UserAuthTokenDetail{}, err
	}
	span.SetAttributes(
		attribute.String("user.id", tok.UserID.String()),
		attribute.String("token.id", tok.ID.String()),
	)

	// Claim before creating the session so a concurrent redeem cannot mint
	// two sessions from the same QR code.
	if err := svc.repos.QRLoginTokens.MarkUsed(ctx, tok.ID, time.Now()); err != nil {
		if errors.Is(err, repo.ErrQRLoginTokenAlreadyClaimed) {
			span.SetAttributes(attribute.String("qr_login.outcome", "claim_race_lost"))
			return UserAuthTokenDetail{}, ErrorQRLoginInvalid
		}
		recordServiceSpanError(span, err)
		span.SetAttributes(attribute.String("qr_login.outcome", "claim_failed"))
		return UserAuthTokenDetail{}, err
	}

	session, err := svc.createSessionToken(ctx, tok.UserID, extendedSession)
	if err != nil {
		recordServiceSpanError(span, err)
		span.SetAttributes(attribute.String("qr_login.outcome", "session_failed"))
		return UserAuthTokenDetail{}, err
	}

	span.SetAttributes(attribute.String("qr_login.outcome", "success"))
	return session, nil
}
