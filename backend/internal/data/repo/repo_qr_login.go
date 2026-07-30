package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/qrlogintokens"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type QRLoginTokenRepository struct {
	db *ent.Client
}

type QRLoginToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
}

// ErrQRLoginTokenAlreadyClaimed is returned when a concurrent redeem wins the
// race, or when the token expired between lookup and claim.
var ErrQRLoginTokenAlreadyClaimed = errors.New("qr login token was already claimed")

// Create persists a hashed QR login token for the given user.
func (r *QRLoginTokenRepository) Create(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (QRLoginToken, error) {
	ctx, span := entityTracer().Start(ctx, "repo.QRLoginTokenRepository.Create",
		trace.WithAttributes(
			attribute.String("user.id", userID.String()),
			attribute.String("token.expires_at", expiresAt.Format(time.RFC3339)),
		))
	defer span.End()

	row, err := r.db.QRLoginTokens.Create().
		SetUserID(userID).
		SetToken(tokenHash).
		SetExpiresAt(expiresAt).
		Save(ctx)
	if err != nil {
		recordSpanError(span, err)
		return QRLoginToken{}, err
	}

	return QRLoginToken{
		ID:        row.ID,
		UserID:    userID,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

// InvalidateUnusedByUser marks every unused, unexpired QR login token for the
// user as used. Called before minting a new token so only one QR is live.
func (r *QRLoginTokenRepository) InvalidateUnusedByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	ctx, span := entityTracer().Start(ctx, "repo.QRLoginTokenRepository.InvalidateUnusedByUser",
		trace.WithAttributes(attribute.String("user.id", userID.String())))
	defer span.End()

	now := time.Now()
	affected, err := r.db.QRLoginTokens.Update().
		Where(
			qrlogintokens.UserID(userID),
			qrlogintokens.UsedAtIsNil(),
			qrlogintokens.ExpiresAtGT(now),
		).
		SetUsedAt(now).
		Save(ctx)
	if err != nil {
		recordSpanError(span, err)
		return 0, err
	}
	span.SetAttributes(attribute.Int("tokens.invalidated.count", affected))
	return affected, nil
}

// GetValidByHash returns the token row matching the given hash if it has not
// expired and has not been used.
func (r *QRLoginTokenRepository) GetValidByHash(ctx context.Context, tokenHash []byte) (QRLoginToken, error) {
	ctx, span := entityTracer().Start(ctx, "repo.QRLoginTokenRepository.GetValidByHash",
		trace.WithAttributes(attribute.Int("token.hash.length", len(tokenHash))))
	defer span.End()

	row, err := r.db.QRLoginTokens.Query().
		Where(
			qrlogintokens.Token(tokenHash),
			qrlogintokens.UsedAtIsNil(),
			qrlogintokens.ExpiresAtGT(time.Now()),
		).
		Only(ctx)
	if err != nil {
		span.SetAttributes(
			attribute.Bool("token.found", false),
			attribute.Bool("token.lookup.not_found", ent.IsNotFound(err)),
		)
		if !ent.IsNotFound(err) {
			recordSpanError(span, err)
		}
		return QRLoginToken{}, err
	}

	span.SetAttributes(attribute.Bool("token.found", true))
	return QRLoginToken{
		ID:        row.ID,
		UserID:    row.UserID,
		ExpiresAt: row.ExpiresAt,
	}, nil
}

// MarkUsed atomically sets used_at on the token when still unused and unexpired.
func (r *QRLoginTokenRepository) MarkUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	ctx, span := entityTracer().Start(ctx, "repo.QRLoginTokenRepository.MarkUsed",
		trace.WithAttributes(attribute.String("token.id", id.String())))
	defer span.End()

	affected, err := r.db.QRLoginTokens.Update().
		Where(
			qrlogintokens.ID(id),
			qrlogintokens.UsedAtIsNil(),
			qrlogintokens.ExpiresAtGT(at),
		).
		SetUsedAt(at).
		Save(ctx)
	if err != nil {
		recordSpanError(span, err)
		return err
	}
	span.SetAttributes(attribute.Int("tokens.claimed.count", affected))
	if affected == 0 {
		return ErrQRLoginTokenAlreadyClaimed
	}
	return nil
}

// PurgeExpired deletes expired and already-used tokens. Run periodically.
func (r *QRLoginTokenRepository) PurgeExpired(ctx context.Context) (int, error) {
	ctx, span := entityTracer().Start(ctx, "repo.QRLoginTokenRepository.PurgeExpired")
	defer span.End()

	deleted, err := r.db.QRLoginTokens.Delete().
		Where(qrlogintokens.Or(
			qrlogintokens.ExpiresAtLTE(time.Now()),
			qrlogintokens.UsedAtNotNil(),
		)).
		Exec(ctx)
	if err != nil {
		recordSpanError(span, err)
		return 0, err
	}
	span.SetAttributes(attribute.Int("tokens.deleted.count", deleted))
	return deleted, nil
}
