export interface Transaction {
	id: number;
	mail_id: number;
	amount: number;
	balance: number;
	type: "add" | "subtract";
	description: string;
	completed: boolean;
	timestamp: string;
	tags?: Tag[];
	splits?: TransactionSplit[];
}

export interface Mail {
	id: number;
	uid: number;
	subject: string;
	from: string;
	to: string;
	timestamp: string;
}

export interface User {
	id: number;
	name: string;
	email: string;
	whitelist: boolean;
	created_at: string;
}

export interface Tag {
	id: number;
	name: string;
	color?: string;
}

export interface TransactionSplit {
	id: number;
	transaction_id: number;
	user_id: number;
	amount: number;
	completed: boolean;
	reason?: string;
	user?: User;
	split_hash?: string;
	created_at: Date;
}

export interface TelegramUser {
	id: number;
	chat_id: number;
	email: string;
	created_at: string;
}

export interface Statistics {
	totalSpent: number;
	totalReceived: number;
	balance: number;
	transactionCount: number;
	spendingByMonth: MonthlySpending[];
	spendingByTag: TagSpending[];
	pendingSplits: number;
	completedSplits: number;
}

export interface MonthlySpending {
	month: string;
	amount: number;
	count: number;
}

export interface TagSpending {
	tag: string;
	amount: number;
	count: number;
}

export interface ReminderRequest {
	split_ids: number[];
	angry?: boolean;
}

export interface BillSplitRequest {
	transaction_id: number;
	users: {
		user_id: number;
		amount: number;
		reason?: string;
	}[];
}

export interface ApiResponse<T> {
	data?: T;
	error?: string;
	message?: string;
}

export interface UserSplitSummary {
	user: User;
	splits: TransactionSplit[];
	total_amount: number;
	bill_count: number;
}
