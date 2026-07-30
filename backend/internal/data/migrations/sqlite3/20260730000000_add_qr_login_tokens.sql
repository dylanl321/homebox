-- +goose Up
-- Short-lived single-use tokens for QR-code device login. Tokens are stored
-- as a sha256 hash; the raw value is shown only in the QR payload. used_at is
-- set when consumed so a replay finds neither a fresh row nor an unmarked one.
create table if not exists qr_login_tokens
(
    id         uuid     not null
        primary key,
    created_at datetime not null,
    updated_at datetime not null,
    user_id    uuid     not null
        constraint qr_login_tokens_users_qr_login_tokens
            references users
            on delete cascade,
    token      blob     not null,
    expires_at datetime not null,
    used_at    datetime
);

create unique index if not exists qr_login_tokens_token_key
    on qr_login_tokens (token);

create index if not exists qrlogintokens_user_id
    on qr_login_tokens (user_id);

-- +goose Down
drop table if exists qr_login_tokens;
