CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    amount INTEGER NOT NULL,
    current_balance INTEGER NOT NULL,
    type TEXT NOT NULL,
    from_account TEXT NOT NULL,
    description TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    completed BOOLEAN DEFAULT 0,
    created_at DATETIME NOT NULL
);