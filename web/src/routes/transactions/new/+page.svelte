<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { formatCurrency } from '$lib/utils/format';

	let amount = $state('');
	let displayAmount = $state('');
	let description = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);

	function handleKeyDown(e: KeyboardEvent) {
		// Allow: backspace, delete, tab, escape, enter
		if ([8, 9, 27, 13, 46].includes(e.keyCode)) {
			return;
		}
		// Allow: Ctrl+A, Ctrl+C, Ctrl+V, Ctrl+X
		if ((e.ctrlKey || e.metaKey) && [65, 67, 86, 88].includes(e.keyCode)) {
			return;
		}
		// Allow: home, end, left, right, up, down
		if (e.keyCode >= 35 && e.keyCode <= 40) {
			return;
		}
		// Ensure that it is a number and stop the keypress if not
		if ((e.shiftKey || e.keyCode < 48 || e.keyCode > 57) && (e.keyCode < 96 || e.keyCode > 105)) {
			e.preventDefault();
		}
	}

	function handleAmountInput(e: Event) {
		const input = e.target as HTMLInputElement;
		// Only allow digits (no decimal separator)
		const value = input.value.replace(/[^\d]/g, '');

		if (value === '') {
			amount = '';
			displayAmount = '';
			return;
		}

		// Store the raw numeric value
		amount = value;

		// Format for display with thousand separators
		const numValue = parseInt(value, 10);
		displayAmount = formatCurrency(numValue).replace('₫', '').trim();

		// Update cursor position to the end after formatting
		setTimeout(() => {
			input.selectionStart = input.selectionEnd = input.value.length;
		}, 0);
	}

	async function handleSubmit() {
		if (!amount || !description) {
			error = 'Please fill in all fields';
			return;
		}

		const amountNum = parseInt(amount, 10);
		if (isNaN(amountNum) || amountNum <= 0) {
			error = 'Please enter a valid amount';
			return;
		}

		loading = true;
		error = null;

		const response = await api.createVirtualBill(amountNum, description);
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			goto(`/transactions/${response.data.id}`);
		}

		loading = false;
	}
</script>

<div class="space-y-6 max-w-2xl mx-auto">
	<div class="flex items-center gap-4">
		<a href="/transactions" class="text-gray-600 hover:text-gray-900" aria-label="Back to transactions">
			<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
			</svg>
		</a>
		<h1 class="text-3xl font-bold text-gray-900">Create Virtual Bill</h1>
	</div>

	<div class="card">
		<p class="text-gray-600 mb-6">
			Create a virtual bill for expenses that aren't automatically tracked from your bank emails.
		</p>

		{#if error}
			<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-6">
				{error}
			</div>
		{/if}

		<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-6">
			<div>
				<label for="description" class="label">Description *</label>
				<input
					id="description"
					type="text"
					bind:value={description}
					placeholder="e.g., Dinner at restaurant, Uber ride, Coffee shop"
					required
					class="input"
				/>
				<p class="text-sm text-gray-500 mt-1">
					A brief description of what this expense is for
				</p>
			</div>

			<div>
				<label for="amount" class="label">Amount (VND) *</label>
				<div class="relative">
					<input
						id="amount"
						type="text"
						inputmode="numeric"
						value={displayAmount}
						oninput={handleAmountInput}
						onkeydown={handleKeyDown}
						placeholder="0"
						required
						class="input pr-12"
					/>
					<span class="absolute right-4 top-1/2 -translate-y-1/2 text-gray-500 pointer-events-none">
						₫
					</span>
				</div>
				<p class="text-sm text-gray-500 mt-1">
					Enter the total amount in Vietnamese Dong
				</p>
			</div>

			<div class="bg-blue-50 border border-blue-200 rounded-lg p-4">
				<div class="flex items-start">
					<svg
						class="w-5 h-5 text-blue-600 mr-2 mt-0.5"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
						/>
					</svg>
					<div class="flex-1">
						<p class="text-sm text-blue-800 font-medium">What happens next?</p>
						<p class="text-sm text-blue-700 mt-1">
							After creating the virtual bill, you'll be able to:
						</p>
						<ul class="text-sm text-blue-700 mt-2 space-y-1 ml-4 list-disc">
							<li>Split the bill among multiple users</li>
							<li>Add tags to categorize the expense</li>
							<li>Send payment reminders</li>
							<li>Track payment status</li>
						</ul>
					</div>
				</div>
			</div>

			<div class="flex gap-3">
				<button
					type="submit"
					disabled={loading}
					class="btn btn-primary flex-1 disabled:opacity-50"
				>
					{loading ? 'Creating...' : 'Create Virtual Bill'}
				</button>
				<a href="/transactions" class="btn btn-secondary">Cancel</a>
			</div>
		</form>
	</div>

	<!-- Quick Actions -->
	<div class="card">
		<h2 class="text-lg font-bold text-gray-900 mb-4">Quick Tips</h2>
		<div class="space-y-3 text-sm text-gray-600">
			<div class="flex items-start">
				<svg class="w-5 h-5 text-primary-600 mr-2 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<p>Virtual bills work the same as automatically tracked transactions</p>
			</div>
			<div class="flex items-start">
				<svg class="w-5 h-5 text-primary-600 mr-2 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<p>Use descriptive names so you can easily find transactions later</p>
			</div>
			<div class="flex items-start">
				<svg class="w-5 h-5 text-primary-600 mr-2 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				<p>You can add tags immediately after creation to categorize the expense</p>
			</div>
		</div>
	</div>
</div>
