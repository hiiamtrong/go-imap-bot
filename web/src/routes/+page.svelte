<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { transactions, totalBalance } from '$lib/stores/transactions';
	import { formatCurrency, formatAmount, formatRelativeTime } from '$lib/utils/format';
	import type { Transaction } from '$lib/types';

	let loading = $state(true);
	let error = $state<string | null>(null);
	let recentTransactions = $state<Transaction[]>([]);
	let stats = $state({
		pendingSplits: 0,
		completedSplits: 0,
		totalTransactions: 0
	});

	onMount(async () => {
		await loadDashboard();
	});

	async function loadDashboard() {
		loading = true;
		error = null;

		// Load recent transactions
		const txResponse = await api.getTransactions(5, 0);
		if (txResponse.error) {
			error = txResponse.error;
		} else if (txResponse.data) {
			recentTransactions = txResponse.data;
			transactions.set(txResponse.data);
		}

		// Load statistics
		const statsResponse = await api.getStatistics();
		if (statsResponse.data) {
			stats = {
				pendingSplits: statsResponse.data.pendingSplits,
				completedSplits: statsResponse.data.completedSplits,
				totalTransactions: statsResponse.data.transactionCount
			};
		}

		loading = false;
	}

	function getTransactionTypeColor(type: 'add' | 'subtract'): string {
		return type === 'add' ? 'text-green-600' : 'text-red-600';
	}
</script>

<div class="space-y-6">
	<div class="flex justify-between items-center">
		<h1 class="text-3xl font-bold text-gray-900">Dashboard</h1>
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

	<!-- Stats Cards -->
	<div class="grid grid-cols-1 md:grid-cols-4 gap-6">
		<div class="card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm text-gray-600">Current Balance</p>
					<p class="text-2xl font-bold text-gray-900">{formatCurrency($totalBalance)}</p>
				</div>
				<div class="bg-primary-100 p-3 rounded-full">
					<svg class="w-6 h-6 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
						/>
					</svg>
				</div>
			</div>
		</div>

		<div class="card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm text-gray-600">Total Transactions</p>
					<p class="text-2xl font-bold text-gray-900">{stats.totalTransactions}</p>
				</div>
				<div class="bg-blue-100 p-3 rounded-full">
					<svg class="w-6 h-6 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
						/>
					</svg>
				</div>
			</div>
		</div>

		<div class="card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm text-gray-600">Pending Splits</p>
					<p class="text-2xl font-bold text-orange-600">{stats.pendingSplits}</p>
				</div>
				<div class="bg-orange-100 p-3 rounded-full">
					<svg class="w-6 h-6 text-orange-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
						/>
					</svg>
				</div>
			</div>
		</div>

		<div class="card">
			<div class="flex items-center justify-between">
				<div>
					<p class="text-sm text-gray-600">Completed Splits</p>
					<p class="text-2xl font-bold text-green-600">{stats.completedSplits}</p>
				</div>
				<div class="bg-green-100 p-3 rounded-full">
					<svg class="w-6 h-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
					</svg>
				</div>
			</div>
		</div>
	</div>

	<!-- Recent Transactions -->
	<div class="card">
		<div class="flex justify-between items-center mb-4">
			<h2 class="text-xl font-bold text-gray-900">Recent Transactions</h2>
			<a href="/transactions" class="text-primary-600 hover:text-primary-700 text-sm font-medium">
				View All →
			</a>
		</div>

		{#if loading}
			<div class="text-center py-8">
				<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
				<p class="mt-2 text-gray-600">Loading transactions...</p>
			</div>
		{:else if recentTransactions.length === 0}
			<p class="text-gray-500 text-center py-8">No transactions yet</p>
		{:else}
			<div class="space-y-3">
				{#each recentTransactions as transaction}
					<a
						href="/transactions/{transaction.id}"
						class="block p-4 border border-gray-200 rounded-lg hover:border-primary-300 hover:shadow-md transition-all"
					>
						<div class="flex justify-between items-start">
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
								<p class="text-sm text-gray-500 mt-1">
									{formatRelativeTime(transaction.timestamp)}
								</p>
								{#if transaction.tags && transaction.tags.length > 0}
									<div class="flex gap-2 mt-2">
										{#each transaction.tags as tag}
											<span class="px-2 py-1 bg-primary-100 text-primary-700 text-xs rounded-full">
												{tag.name}
											</span>
										{/each}
									</div>
								{/if}
							</div>
							<div class="text-right">
								<p class="text-lg font-bold {getTransactionTypeColor(transaction.type)}">
									{formatAmount(transaction.amount, transaction.type)}
								</p>
								<p class="text-sm text-gray-500 mt-1">
									Balance: {formatCurrency(transaction.balance)}
								</p>
								{#if transaction.splits && transaction.splits.length > 0}
									<p class="text-xs text-gray-500 mt-1">
										{transaction.splits.filter((s) => !s.completed).length} pending splits
									</p>
								{/if}
							</div>
						</div>
					</a>
				{/each}
			</div>
		{/if}
	</div>
</div>
