CREATE TABLE IF NOT EXISTS mails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uid INTEGER,
		subject TEXT,
		from_account TEXT,
		to_account TEXT,
		timestamp INTEGER
);

CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT,
		mail_id INTEGER,
		amount INTEGER,
        current_balance INTEGER,
        from_account TEXT,
        to_account TEXT,
        description TEXT,
        timestamp INTEGER,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        email TEXT,
        name TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS transaction_splits (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        transaction_id INTEGER,
        user_id INTEGER,
        amount INTEGER,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (transaction_id) REFERENCES transactions(id),
        FOREIGN KEY (user_id) REFERENCES users(id)
);




