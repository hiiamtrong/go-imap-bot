import { format, formatDistanceToNow, parseISO } from 'date-fns';

export function formatCurrency(amount: number): string {
	return new Intl.NumberFormat('vi-VN', {
		style: 'currency',
		currency: 'VND'
	}).format(amount);
}

export function formatDate(date: string | Date): string {
	const dateObj = typeof date === 'string' ? parseISO(date) : date;
	return format(dateObj, 'MMM dd, yyyy HH:mm');
}

export function formatRelativeTime(date: string | Date): string {
	const dateObj = typeof date === 'string' ? parseISO(date) : date;
	return formatDistanceToNow(dateObj, { addSuffix: true });
}

export function formatAmount(amount: number, type: 'add' | 'subtract'): string {
	const formatted = formatCurrency(Math.abs(amount));
	return type === 'add' ? `+${formatted}` : `-${formatted}`;
}
