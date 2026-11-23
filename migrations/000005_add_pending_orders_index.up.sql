CREATE INDEX gophermart_orders_pending_id_idx ON public.gophermart_orders (id) WHERE status IN ('NEW', 'PROCESSING');
