ALTER TABLE users ADD COLUMN subscription_plan TEXT NOT NULL DEFAULT 'free';
ALTER TABLE users ADD COLUMN creem_subscription_id TEXT;
ALTER TABLE users ADD COLUMN creem_customer_id TEXT;
