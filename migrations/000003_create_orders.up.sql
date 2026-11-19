BEGIN;

CREATE TABLE public.gophermart_orders (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    user_id BIGINT NOT NULL,

    number TEXT NOT NULL,

    status TEXT NOT NULL DEFAULT 'NEW',

    accrual BIGINT,

    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    CONSTRAINT gophermart_orders_user_fk
        FOREIGN KEY (user_id)
        REFERENCES public.gophermart_users (id)
        ON DELETE CASCADE,

    CONSTRAINT gophermart_orders_number_key UNIQUE (number),

    CONSTRAINT gophermart_orders_status_check
        CHECK (status IN ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED')),

    CONSTRAINT gophermart_orders_accrual_check CHECK (
        (status IN ('NEW', 'PROCESSING') AND accrual IS NULL)
        OR (status = 'INVALID' AND accrual IS NOT NULL AND accrual = 0)
        OR (status = 'PROCESSED' AND accrual IS NOT NULL AND accrual >= 0)
    )
);

CREATE INDEX gophermart_orders_user_uploaded_id_idx
    ON public.gophermart_orders (user_id, uploaded_at DESC, id DESC);

COMMIT;
