BEGIN;

CREATE TABLE public.gophermart_withdrawals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    user_id BIGINT NOT NULL,

    order_number TEXT NOT NULL,

    sum BIGINT NOT NULL,

    processed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT gophermart_withdrawals_user_fk
        FOREIGN KEY (user_id)
        REFERENCES public.gophermart_users (id)
        ON DELETE CASCADE,

    CONSTRAINT gophermart_withdrawals_order_number_key UNIQUE (order_number),
    CONSTRAINT gophermart_withdrawals_sum_check CHECK (sum > 0)
);

CREATE INDEX gophermart_withdrawals_user_processed_id_idx
    ON public.gophermart_withdrawals (user_id, processed_at DESC, id DESC);

COMMIT;
