package v1

import (
	"errors"
	"net/http"
	"time"

	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
	"go.opentelemetry.io/otel/attribute"
)

type (
	QRLoginCreateResponse struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expiresAt"`
	}

	QRLoginExchangeRequest struct {
		Token        string `json:"token"        validate:"required" example:"ABCDEFGHIJKLMNOPQRSTUVWXYZ"`
		StayLoggedIn bool   `json:"stayLoggedIn"`
	}
)

// HandleQRLoginCreate godoc
//
//	@Summary		Create QR Login Token
//	@Description	Mints a short-lived single-use token for logging in another device via QR code.
//	@Description	Any previously unused QR login token for this user is invalidated.
//	@Tags			Authentication
//	@Produce		json
//	@Success		200	{object}	QRLoginCreateResponse
//	@Failure		401	{string}	string	"unauthorized"
//	@Router			/v1/users/self/qr-login [POST]
//	@Security		Bearer
func (ctrl *V1Controller) HandleQRLoginCreate() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		spanCtx, span := startEntityCtrlSpan(r.Context(), "controller.V1.HandleQRLoginCreate")
		defer span.End()

		actor := services.UseUserCtx(spanCtx)
		if actor == nil {
			span.SetAttributes(attribute.String("qr_login.outcome", "no_user"))
			return validate.NewUnauthorizedError()
		}
		span.SetAttributes(attribute.String("user.id", actor.ID.String()))

		detail, err := ctrl.svc.User.CreateQRLoginToken(spanCtx, actor.ID)
		if err != nil {
			recordCtrlSpanError(span, err)
			span.SetAttributes(attribute.String("qr_login.outcome", "create_failed"))
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		span.SetAttributes(
			attribute.String("qr_login.outcome", "created"),
			attribute.String("token.expires_at", detail.ExpiresAt.Format(time.RFC3339)),
		)
		return server.JSON(w, http.StatusOK, QRLoginCreateResponse{
			Token:     detail.Token,
			ExpiresAt: detail.ExpiresAt,
		})
	}
}

// HandleQRLoginExchange godoc
//
//	@Summary		Exchange QR Login Token
//	@Description	Consumes a QR login token and establishes a session for the owning user.
//	@Tags			Authentication
//	@Accept			application/json
//	@Produce		json
//	@Param			payload	body		QRLoginExchangeRequest	true	"QR login token"
//	@Success		200		{object}	TokenResponse
//	@Failure		400		{string}	string	"missing or invalid request body"
//	@Failure		401		{string}	string	"invalid or expired QR login token"
//	@Router			/v1/users/login/qr [POST]
func (ctrl *V1Controller) HandleQRLoginExchange() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		spanCtx, span := startEntityCtrlSpan(r.Context(), "controller.V1.HandleQRLoginExchange")
		defer span.End()

		var body QRLoginExchangeRequest
		if err := server.Decode(r, &body); err != nil {
			span.SetAttributes(attribute.String("qr_login.outcome", "decode_failed"))
			return validate.NewRequestError(err, http.StatusBadRequest)
		}
		span.SetAttributes(
			attribute.Int("token.length", len(body.Token)),
			attribute.Bool("session.extended", body.StayLoggedIn),
		)

		newToken, err := ctrl.svc.User.ExchangeQRLoginToken(spanCtx, body.Token, body.StayLoggedIn)
		if err != nil {
			if errors.Is(err, services.ErrorQRLoginInvalid) {
				span.SetAttributes(attribute.String("qr_login.outcome", "token_invalid"))
				return validate.NewUnauthorizedError()
			}
			recordCtrlSpanError(span, err)
			span.SetAttributes(attribute.String("qr_login.outcome", "exchange_failed"))
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		span.SetAttributes(
			attribute.String("qr_login.outcome", "success"),
			attribute.String("auth.session.expires_at", newToken.ExpiresAt.Format(time.RFC3339)),
		)
		ctrl.setCookies(w, noPort(r.Host), newToken.Raw, newToken.ExpiresAt, body.StayLoggedIn, newToken.AttachmentToken)
		return server.JSON(w, http.StatusOK, TokenResponse{
			Token:           "Bearer " + newToken.Raw,
			ExpiresAt:       newToken.ExpiresAt,
			AttachmentToken: newToken.AttachmentToken,
		})
	}
}
