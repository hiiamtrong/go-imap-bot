import { format, formatDistanceToNow, parseISO } from "date-fns";

// Currency symbols mapping
const currencySymbols: Record<string, string> = {
	VND: "₫",
	USD: "$",
	EUR: "€",
	GBP: "£",
	JPY: "¥",
	CNY: "¥",
	KRW: "₩",
	THB: "฿",
	SGD: "S$",
	AUD: "A$",
};

export function formatCurrency(amount: number, currency: string): string {
	const symbol = currencySymbols[currency];

	// Format number with thousand separators
	const formattedNumber = new Intl.NumberFormat("vi-VN").format(Math.abs(amount));

	if (!symbol) {
		// If currency not in map, use the currency code itself
		return `${formattedNumber} ${currency}`;
	}

	// For VND, put symbol after the number (Vietnamese style)
	if (currency === "VND") {
		return `${formattedNumber}${symbol}`;
	}

	// For other currencies, put symbol before the number
	return `${symbol}${formattedNumber}`;
}

export function formatDate(date: string | Date): string {
	const dateObj = typeof date === "string" ? parseISO(date) : date;
	return format(dateObj, "MMM dd, yyyy HH:mm");
}

export function formatRelativeTime(date: string | Date): string {
	const dateObj = typeof date === "string" ? parseISO(date) : date;
	return formatDistanceToNow(dateObj, { addSuffix: true });
}

export function formatAmount(amount: number, type: "add" | "subtract", currency: string): string {
	const formatted = formatCurrency(Math.abs(amount), currency);
	return type === "add" ? `+${formatted}` : `-${formatted}`;
}
