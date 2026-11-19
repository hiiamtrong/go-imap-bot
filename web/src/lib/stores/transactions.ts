import { writable, derived } from "svelte/store";
import type { Transaction } from "$lib/types";

export const transactions = writable<Transaction[]>([]);
export const selectedTransaction = writable<Transaction | null>(null);
export const loading = writable<boolean>(false);
export const error = writable<string | null>(null);

// Query string ("?type=add&page=2") of the transactions list, so the detail page can link back to it.
export const listQuery = writable("");

// Derived stores
export const pendingTransactions = derived(transactions, ($transactions) =>
	$transactions.filter((t) => t.splits?.some((s) => !s.completed))
);

export const completedTransactions = derived(transactions, ($transactions) =>
	$transactions.filter((t) => t.splits?.every((s) => s.completed))
);

export const totalBalance = derived(transactions, ($transactions) => {
	if ($transactions.length === 0) return 0;
	return $transactions[0]?.balance || 0;
});
