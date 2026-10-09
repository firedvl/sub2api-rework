ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS bonus_amount DECIMAL(20, 2) NOT NULL DEFAULT 0;

ALTER TABLE payment_orders
    ALTER COLUMN pay_amount TYPE DECIMAL(21, 3);
