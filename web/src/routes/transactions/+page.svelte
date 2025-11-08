<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { formatCurrency, formatDate, formatAmount, formatRelativeTime } from '$lib/utils/format';
	import type { Transaction } from '$lib/types';

	let transactions = $state<Transaction[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let filterType = $state<'all' | 'add' | 'subtract'>('all');
	let searchQuery = $state('');

	onMount(async () => {
		await loadTransactions();
	});

	async function loadTransactions() {
		loading = true;
		error = null;

		const response = await api.getTransactions(50, 0);
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			transactions = response.data;
		}

		loading = false;
	}

	function getTransactionTypeColor(type: 'add' | 'subtract'): string {
		return type === 'add' ? 'text-green-600' : 'text-red-600';
	}

	const filteredTransactions = $derived(() => {
		let result = transactions;

		if (filterType !== 'all') {
			result = result.filter((t) => t.type === filterType);
		}

		if (searchQuery) {
			result = result.filter((t) =>
				t.description.toLowerCase().includes(searchQuery.toLowerCase())
			);
		}

		return result;
	});
</script>

<div class="space-y-6">
	<div class="flex justify-between items-center">
		<h1 class="text-3xl font-bold text-gray-900">Transactions</h1>
		<a href="/transactions/new" class="btn btn-primary">
			<svg class="w-5 h-5 inline-block mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M12 4v16m8-8H4"
				/>
			</svg>
			Add Virtual Bill
		</a>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	<!-- Filters -->
	<div class="card">
		<div class="flex flex-col md:flex-row gap-4">
			<div class="flex-1">
				<input
					type="text"
					placeholder="Search transactions..."
					bind:value={searchQuery}
					class="input"
				/>
			</div>
			<div class="flex gap-2">
				<button
					onclick={() => (filterType = 'all')}
					class="btn {filterType === 'all' ? 'btn-primary' : 'btn-secondary'}"
				>
					All
				</button>
				<button
					onclick={() => (filterType = 'add')}
					class="btn {filterType === 'add' ? 'btn-primary' : 'btn-secondary'}"
				>
					Income
				</button>
				<button
					onclick={() => (filterType = 'subtract')}
					class="btn {filterType === 'subtract' ? 'btn-primary' : 'btn-secondary'}"
				>
					Expense
				</button>
			</div>
		</div>
	</div>

	<!-- Transactions List -->
	<div class="card">
		{#if loading}
			<div class="text-center py-8">
				<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
				<p class="mt-2 text-gray-600">Loading transactions...</p>
			</div>
		{:else if filteredTransactions().length === 0}
			<p class="text-gray-500 text-center py-8">No transactions found</p>
		{:else}
			<div class="space-y-3">
				{#each filteredTransactions() as transaction}
					<a
						href="/transactions/{transaction.id}"
						class="block p-4 border border-gray-200 rounded-lg hover:border-primary-300 hover:shadow-md transition-all"
					>
						<div class="flex justify-between items-start">
							<div class="flex-1">
								<div class="flex items-center gap-3">
									<div
										class="w-10 h-10 rounded-full flex items-center justify-center
											{transaction.type === 'add' ? 'bg-green-100' : 'bg-red-100'}"
									>
										{#if transaction.type === 'add'}
											<svg
												class="w-5 h-5 text-green-600"
												fill="none"
												stroke="currentColor"
												viewBox="0 0 24 24"
											>
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M12 4v16m0-16l-4 4m4-4l4 4"
												/>
											</svg>
										{:else}
											<svg
												class="w-5 h-5 text-red-600"
												fill="none"
												stroke="currentColor"
												viewBox="0 0 24 24"
											>
												<path
													stroke-linecap="round"
													stroke-linejoin="round"
													stroke-width="2"
													d="M12 20V4m0 16l-4-4m4 4l4-4"
												/>
											</svg>
										{/if}
									</div>
									<div class="flex-1">
										<div class="flex items-center gap-2">
											<p class="font-medium text-gray-900">{transaction.description}</p>
											{#if transaction.completed}
												<span class="inline-flex items-center px-2 py-0.5 bg-green-100 text-green-700 text-xs font-medium rounded-full">
													<svg class="w-3 h-3 mr-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
														<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
													</svg>
													Completed
												</span>
											{/if}
										</div>
										<p class="text-sm text-gray-500">
											{formatRelativeTime(transaction.timestamp)} • {formatDate(transaction.timestamp)}
										</p>
									</div>
								</div>
								{#if transaction.tags && transaction.tags.length > 0}
									<div class="flex gap-2 mt-3 ml-13">
										{#each transaction.tags as tag}
											<span class="px-2 py-1 bg-primary-100 text-primary-700 text-xs rounded-full">
												{tag.name}
											</span>
										{/each}
									</div>
								{/if}
							</div>
							<div class="text-right ml-4">
								<p class="text-xl font-bold {getTransactionTypeColor(transaction.type)}">
									{formatAmount(transaction.amount, transaction.type)}
								</p>
								<p class="text-sm text-gray-500 mt-1">
									Balance: {formatCurrency(transaction.balance)}
								</p>
								{#if transaction.splits && transaction.splits.length > 0}
									<div class="mt-2">
										<span
											class="px-2 py-1 text-xs rounded-full
											{transaction.splits.some((s) => !s.completed)
												? 'bg-orange-100 text-orange-700'
												: 'bg-green-100 text-green-700'}"
										>
											{transaction.splits.filter((s) => !s.completed).length === 0
												? 'All paid'
												: `${transaction.splits.filter((s) => !s.completed).length} pending`}
										</span>
									</div>
								{/if}
							</div>
						</div>
					</a>
				{/each}
			</div>
		{/if}
	</div>
</div>
