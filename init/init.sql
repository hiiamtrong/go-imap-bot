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



