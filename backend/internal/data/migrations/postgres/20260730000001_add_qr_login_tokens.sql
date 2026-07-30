-- +goose Up
-- Short-lived single-use tokens for QR-code device login. Tokens are stored
-- as a sha256 hash; the raw value is shown only in the QR payload. used_at is
-- set when consumed so a replay finds neither a fresh row nor an unmarked one.
CREATE TABLE IF NOT EXISTS "qr_login_tokens" (
    "id"         uuid NOT NULL,
    "created_at" timestamptz NOT NULL,
    "updated_at" timestamptz NOT NULL,
    "user_id"    uuid NOT NULL,
    "token"      bytea NOT NULL,
    "expires_at" timestamptz NOT NULL,
    "used_at"    timestamptz NULL,
    PRIMARY KEY ("id"),
    CONSTRAINT "qr_login_tokens_users_qr_login_tokens" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS "qr_login_tokens_token_key" ON "qr_login_tokens" ("token");
CREATE INDEX IF NOT EXISTS "qrlogintokens_user_id" ON "qr_login_tokens" ("user_id");

-- +goose Down
DROP TABLE IF EXISTS "qr_login_tokens";
