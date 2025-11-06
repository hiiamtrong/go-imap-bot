<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { formatCurrency } from '$lib/utils/format';
	import type { Transaction, User } from '$lib/types';

	interface Props {
		transaction: Transaction;
		onClose: () => void;
	}

	let { transaction, onClose }: Props = $props();

	let users = $state<User[]>([]);
	let selectedUsers = $state<Set<number>>(new Set());
	let splitMode = $state<'custom' | 'equal'>('equal');
	let customAmounts = $state<Map<number, string>>(new Map());
	let reasons = $state<Map<number, string>>(new Map());
	let loading = $state(false);
	let error = $state<string | null>(null);

	onMount(async () => {
		const response = await api.getUsers();
		if (response.data) {
			users = response.data;
		}
	});

	function toggleUser(userId: number) {
		const newSet = new Set(selectedUsers);
		if (newSet.has(userId)) {
			newSet.delete(userId);
			customAmounts.delete(userId);
			reasons.delete(userId);
		} else {
			newSet.add(userId);
		}
		selectedUsers = newSet;
	}

	function calculateEqualSplit(): number {
		const count = selectedUsers.size;
		if (count === 0) return 0;
		return Math.abs(transaction.amount) / count;
	}

	function calculateTotal(): number {
		if (splitMode === 'equal') {
			return calculateEqualSplit() * selectedUsers.size;
		}

		let total = 0;
		selectedUsers.forEach((userId) => {
			const amount = parseFloat(customAmounts.get(userId) || '0');
			total += amount;
		});
		return total;
	}

	function isValidSplit(): boolean {
		if (selectedUsers.size === 0) return false;
		const total = calculateTotal();
		const transactionAmount = Math.abs(transaction.amount);
		return Math.abs(total - transactionAmount) < 0.01;
	}

	async function handleSubmit() {
		if (!isValidSplit()) {
			error = 'Split amounts must equal transaction amount';
			return;
		}

		loading = true;
		error = null;

		const splits = Array.from(selectedUsers).map((userId) => ({
			user_id: userId,
			amount:
				splitMode === 'equal'
					? calculateEqualSplit()
					: parseFloat(customAmounts.get(userId) || '0'),
			reason: reasons.get(userId)
		}));

		const response = await api.createBillSplit({
			transaction_id: transaction.id,
			users: splits
		});

		if (response.error) {
			error = response.error;
		} else {
			onClose();
		}

		loading = false;
	}
</script>

<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
	<div class="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
		<div class="p-6">
			<div class="flex justify-between items-center mb-6">
				<h2 class="text-2xl font-bold text-gray-900">Split Bill</h2>
				<button onclick={onClose} class="text-gray-500 hover:text-gray-700">
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				</button>
			</div>

			{#if error}
				<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-4">
					{error}
				</div>
			{/if}

			<div class="mb-6">
				<p class="text-gray-600">Transaction Amount:</p>
				<p class="text-2xl font-bold text-gray-900">{formatCurrency(Math.abs(transaction.amount))}</p>
			</div>

			<!-- Split Mode -->
			<div class="mb-6">
				<label class="label">Split Mode</label>
				<div class="flex gap-4">
					<label class="flex items-center">
						<input
							type="radio"
							bind:group={splitMode}
							value="equal"
							class="mr-2"
						/>
						Equal Split
					</label>
					<label class="flex items-center">
						<input
							type="radio"
							bind:group={splitMode}
							value="custom"
							class="mr-2"
						/>
						Custom Amounts
					</label>
				</div>
			</div>

			<!-- User Selection -->
			<div class="mb-6">
				<label class="label">Select Users</label>
				<div class="space-y-2 max-h-60 overflow-y-auto">
					{#each users as user}
						<div class="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
							<label class="flex items-center flex-1 cursor-pointer">
								<input
									type="checkbox"
									checked={selectedUsers.has(user.id)}
									onchange={() => toggleUser(user.id)}
									class="mr-3"
								/>
								<div>
									<p class="font-medium text-gray-900">{user.name}</p>
									<p class="text-sm text-gray-500">{user.email}</p>
								</div>
							</label>

							{#if selectedUsers.has(user.id)}
								<div class="ml-4">
									{#if splitMode === 'equal'}
										<p class="text-sm font-medium text-primary-600">
											{formatCurrency(calculateEqualSplit())}
										</p>
									{:else}
										<input
											type="number"
											placeholder="Amount"
											bind:value={customAmounts[user.id]}
											class="input w-32 text-sm"
										/>
									{/if}
								</div>
							{/if}
						</div>

						{#if selectedUsers.has(user.id)}
							<div class="ml-4">
								<input
									type="text"
									placeholder="Reason (optional)"
									bind:value={reasons[user.id]}
									class="input text-sm"
								/>
							</div>
						{/if}
					{/each}
				</div>
			</div>

			<!-- Summary -->
			{#if selectedUsers.size > 0}
				<div class="mb-6 p-4 bg-gray-50 rounded-lg">
					<div class="flex justify-between mb-2">
						<span>Selected Users:</span>
						<span class="font-medium">{selectedUsers.size}</span>
					</div>
					<div class="flex justify-between mb-2">
						<span>Total Split:</span>
						<span class="font-medium">{formatCurrency(calculateTotal())}</span>
					</div>
					<div class="flex justify-between">
						<span>Transaction Amount:</span>
						<span class="font-medium">{formatCurrency(Math.abs(transaction.amount))}</span>
					</div>
					{#if !isValidSplit() && selectedUsers.size > 0}
						<p class="text-red-600 text-sm mt-2">
							Split amounts must equal transaction amount
						</p>
					{/if}
				</div>
			{/if}

			<!-- Actions -->
			<div class="flex gap-3">
				<button
					onclick={handleSubmit}
					disabled={!isValidSplit() || loading}
					class="btn btn-primary flex-1 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					{loading ? 'Creating...' : 'Create Split'}
				</button>
				<button onclick={onClose} class="btn btn-secondary">Cancel</button>
			</div>
		</div>
	</div>
</div>
