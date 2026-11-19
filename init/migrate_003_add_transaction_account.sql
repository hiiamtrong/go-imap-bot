-- The user's own account/card and the other side of the transfer, as named in the bank mail
ALTER TABLE transactions ADD COLUMN account TEXT DEFAULT '';
ALTER TABLE transactions ADD COLUMN counterparty TEXT DEFAULT '';
