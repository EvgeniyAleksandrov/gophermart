BEGIN;

CREATE TABLE public.gophermart_users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    login TEXT COLLATE "C" NOT NULL,

    password_hash TEXT NOT NULL,

    CONSTRAINT gophermart_users_login_key UNIQUE (login),
    CONSTRAINT gophermart_users_password_hash_check
        CHECK (password_hash ~ '^[0-9a-f]{32}$')
);

COMMIT;
