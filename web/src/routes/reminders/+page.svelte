<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { formatCurrency } from '$lib/utils/format';
	import type { Transaction } from '$lib/types';

	let transactions = $state<Transaction[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let selectedSplits = $state<Set<number>>(new Set());
	let angryMode = $state(false);
	let sending = $state(false);

	onMount(async () => {
		await loadPendingTransactions();
	});

	async function loadPendingTransactions() {
		loading = true;
		error = null;

		const response = await api.getTransactions(50, 0);
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			// Filter to only show transactions with pending splits
			transactions = response.data.filter(
				(t) => t.splits && t.splits.some((s) => !s.completed)
			);
		}

		loading = false;
	}

	function toggleSplit(splitId: number) {
		const newSet = new Set(selectedSplits);
		if (newSet.has(splitId)) {
			newSet.delete(splitId);
		} else {
			newSet.add(splitId);
		}
		selectedSplits = newSet;
	}

	function selectAllPending() {
		const allPending = new Set<number>();
		transactions.forEach((t) => {
			t.splits?.forEach((s) => {
				if (!s.completed) {
					allPending.add(s.id);
				}
			});
		});
		selectedSplits = allPending;
	}

	async function sendReminders() {
		if (selectedSplits.size === 0) {
			error = 'Please select at least one split to send reminders';
			return;
		}

		sending = true;
		error = null;

		const response = await api.sendReminder({
			split_ids: Array.from(selectedSplits),
			angry: angryMode
		});

		if (response.error) {
			error = response.error;
		} else {
			alert(`Successfully sent ${selectedSplits.size} reminder(s)!`);
			selectedSplits = new Set();
			await loadPendingTransactions();
		}

		sending = false;
	}
</script>

<div class="space-y-6">
	<div class="flex justify-between items-center">
		<h1 class="text-3xl font-bold text-gray-900">Payment Reminders</h1>
		<button
			onclick={selectAllPending}
			disabled={loading || transactions.length === 0}
			class="btn btn-secondary disabled:opacity-50"
		>
			Select All Pending
		</button>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	<!-- Reminder Options -->
	<div class="card">
		<h2 class="text-xl font-bold text-gray-900 mb-4">Reminder Settings</h2>
		<div class="flex items-center gap-4">
			<label class="flex items-center cursor-pointer">
				<input type="checkbox" bind:checked={angryMode} class="mr-2" />
				<span class="text-gray-700">
					Use "Angry" reminder template (more assertive)
				</span>
			</label>
		</div>
		<p class="text-sm text-gray-500 mt-2">
			{angryMode ? 'Will send assertive reminder emails' : 'Will send standard reminder emails'}
		</p>
	</div>

	<!-- Pending Splits -->
	<div class="card">
		<div class="flex justify-between items-center mb-4">
			<h2 class="text-xl font-bold text-gray-900">Pending Payments</h2>
			{#if selectedSplits.size > 0}
				<button onclick={sendReminders} disabled={sending} class="btn btn-primary disabled:opacity-50">
					{sending ? 'Sending...' : `Send ${selectedSplits.size} Reminder${selectedSplits.size > 1 ? 's' : ''}`}
				</button>
			{/if}
		</div>

		{#if loading}
			<div class="text-center py-8">
				<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
				<p class="mt-2 text-gray-600">Loading pending payments...</p>
			</div>
		{:else if transactions.length === 0}
			<div class="text-center py-8">
				<svg
					class="w-16 h-16 mx-auto text-green-500 mb-4"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
					/>
				</svg>
				<p class="text-gray-500">All payments are up to date!</p>
			</div>
		{:else}
			<div class="space-y-4">
				{#each transactions as transaction}
					<div class="border border-gray-200 rounded-lg p-4">
						<div class="flex justify-between items-start mb-3">
							<div>
								<h3 class="font-medium text-gray-900">{transaction.description}</h3>
								<p class="text-sm text-gray-500">
									Total: {formatCurrency(Math.abs(transaction.amount))}
								</p>
							</div>
							<a
								href="/transactions/{transaction.id}"
								class="text-primary-600 hover:text-primary-700 text-sm"
							>
								View Details →
							</a>
						</div>

						<div class="space-y-2">
							{#each transaction.splits?.filter((s) => !s.completed) || [] as split}
								<label
									class="flex items-center justify-between p-3 bg-gray-50 rounded cursor-pointer hover:bg-gray-100"
								>
									<div class="flex items-center flex-1">
										<input
											type="checkbox"
											checked={selectedSplits.has(split.id)}
											onchange={() => toggleSplit(split.id)}
											class="mr-3"
										/>
										<div>
											<p class="font-medium text-gray-900">{split.user?.name}</p>
											<p class="text-sm text-gray-500">{split.user?.email}</p>
											{#if split.reason}
												<p class="text-xs text-gray-600 mt-1">{split.reason}</p>
											{/if}
											{#if split.user?.whitelist}
												<span class="inline-block px-2 py-1 bg-yellow-100 text-yellow-700 text-xs rounded-full mt-1">
													Whitelisted
												</span>
											{/if}
										</div>
									</div>
									<div class="text-right">
										<p class="font-bold text-orange-600">{formatCurrency(split.amount)}</p>
									</div>
								</label>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
