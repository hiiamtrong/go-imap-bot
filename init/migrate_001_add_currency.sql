-- Add currency column to transactions and transaction_splits tables
ALTER TABLE transactions ADD COLUMN currency TEXT DEFAULT 'VND';
ALTER TABLE transaction_splits ADD COLUMN currency TEXT DEFAULT 'VND';
UPDATE transactions SET currency = 'VND' WHERE currency IS NULL;
UPDATE transaction_splits SET currency = 'VND' WHERE currency IS NULL;
