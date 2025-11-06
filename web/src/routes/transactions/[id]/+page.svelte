<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { formatCurrency, formatDate, formatAmount } from '$lib/utils/format';
	import type { Transaction, User, Tag, TransactionSplit } from '$lib/types';
	import BillSplitModal from '$lib/components/BillSplitModal.svelte';
	import TagSelector from '$lib/components/TagSelector.svelte';

	let transaction = $state<Transaction | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showSplitModal = $state(false);
	let showTagSelector = $state(false);

	const transactionId = $derived(parseInt($page.params.id));

	onMount(async () => {
		await loadTransaction();
	});

	async function loadTransaction() {
		loading = true;
		error = null;

		const response = await api.getTransaction(transactionId);
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			transaction = response.data;
		}

		loading = false;
	}

	async function handleCompleteSplit(splitId: number) {
		const response = await api.completeSplit(splitId);
		if (response.error) {
			alert('Error: ' + response.error);
		} else {
			await loadTransaction();
		}
	}

	async function handleDeleteSplit(splitId: number) {
		if (!confirm('Are you sure you want to delete this split?')) return;

		const response = await api.deleteSplit(splitId);
		if (response.error) {
			alert('Error: ' + response.error);
		} else {
			await loadTransaction();
		}
	}

	function getTransactionTypeColor(type: 'add' | 'subtract'): string {
		return type === 'add' ? 'text-green-600' : 'text-red-600';
	}
</script>

<div class="space-y-6">
	<div class="flex items-center gap-4">
		<a href="/transactions" class="text-gray-600 hover:text-gray-900">
			<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
			</svg>
		</a>
		<h1 class="text-3xl font-bold text-gray-900">Transaction Details</h1>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	{#if loading}
		<div class="card text-center py-12">
			<div class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
			<p class="mt-4 text-gray-600">Loading transaction...</p>
		</div>
	{:else if transaction}
		<!-- Transaction Info -->
		<div class="card">
			<div class="flex justify-between items-start">
				<div class="flex-1">
					<h2 class="text-2xl font-bold text-gray-900">{transaction.description}</h2>
					<p class="text-gray-500 mt-2">{formatDate(transaction.timestamp)}</p>

					{#if transaction.tags && transaction.tags.length > 0}
						<div class="flex gap-2 mt-4">
							{#each transaction.tags as tag}
								<span class="px-3 py-1 bg-primary-100 text-primary-700 text-sm rounded-full">
									{tag.name}
								</span>
							{/each}
						</div>
					{/if}
				</div>
				<div class="text-right">
					<p class="text-4xl font-bold {getTransactionTypeColor(transaction.type)}">
						{formatAmount(transaction.amount, transaction.type)}
					</p>
					<p class="text-gray-500 mt-2">Balance: {formatCurrency(transaction.balance)}</p>
				</div>
			</div>

			<div class="flex gap-3 mt-6">
				<button onclick={() => (showSplitModal = true)} class="btn btn-primary">
					<svg class="w-5 h-5 inline-block mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
						/>
					</svg>
					Split Bill
				</button>
				<button onclick={() => (showTagSelector = true)} class="btn btn-secondary">
					<svg class="w-5 h-5 inline-block mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"
						/>
					</svg>
					Add Tag
				</button>
			</div>
		</div>

		<!-- Bill Splits -->
		{#if transaction.splits && transaction.splits.length > 0}
			<div class="card">
				<h3 class="text-xl font-bold text-gray-900 mb-4">Bill Splits</h3>
				<div class="space-y-3">
					{#each transaction.splits as split}
						<div class="flex justify-between items-center p-4 bg-gray-50 rounded-lg">
							<div class="flex-1">
								<p class="font-medium text-gray-900">{split.user?.name || 'Unknown User'}</p>
								<p class="text-sm text-gray-500">{split.user?.email}</p>
								{#if split.reason}
									<p class="text-sm text-gray-600 mt-1">{split.reason}</p>
								{/if}
							</div>
							<div class="text-right">
								<p class="text-lg font-bold text-gray-900">{formatCurrency(split.amount)}</p>
								{#if split.completed}
									<span class="inline-flex items-center px-2 py-1 bg-green-100 text-green-700 text-xs rounded-full mt-1">
										<svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
										</svg>
										Paid
									</span>
								{:else}
									<button
										onclick={() => handleCompleteSplit(split.id)}
										class="text-sm text-primary-600 hover:text-primary-700 mt-1"
									>
										Mark as Paid
									</button>
								{/if}
							</div>
							{#if !split.completed}
								<button
									onclick={() => handleDeleteSplit(split.id)}
									class="ml-3 text-red-600 hover:text-red-700"
								>
									<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
										/>
									</svg>
								</button>
							{/if}
						</div>
					{/each}
				</div>

				<div class="mt-4 pt-4 border-t border-gray-200">
					<div class="flex justify-between text-lg font-bold">
						<span>Total Split:</span>
						<span>
							{formatCurrency(transaction.splits.reduce((sum, split) => sum + split.amount, 0))}
						</span>
					</div>
					<div class="flex justify-between text-sm text-gray-600 mt-2">
						<span>Pending:</span>
						<span class="text-orange-600">
							{transaction.splits.filter((s) => !s.completed).length} people
						</span>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</div>

{#if showSplitModal && transaction}
	<BillSplitModal
		{transaction}
		onClose={() => {
			showSplitModal = false;
			loadTransaction();
		}}
	/>
{/if}

{#if showTagSelector && transaction}
	<TagSelector
		{transaction}
		onClose={() => {
			showTagSelector = false;
			loadTransaction();
		}}
	/>
{/if}
