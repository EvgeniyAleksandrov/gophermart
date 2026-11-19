BEGIN;

CREATE TABLE public.gophermart_balances (
    user_id BIGINT PRIMARY KEY,

    current BIGINT NOT NULL DEFAULT 0,

    withdrawn BIGINT NOT NULL DEFAULT 0,

    CONSTRAINT gophermart_balances_user_fk
        FOREIGN KEY (user_id)
        REFERENCES public.gophermart_users (id)
        ON DELETE CASCADE,

    CONSTRAINT gophermart_balances_current_check CHECK (current >= 0),
    CONSTRAINT gophermart_balances_withdrawn_check CHECK (withdrawn >= 0)
);

COMMIT;
