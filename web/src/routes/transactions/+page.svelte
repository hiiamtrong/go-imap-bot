<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { formatCurrency, formatDate, formatAmount, formatRelativeTime } from '$lib/utils/format';
	import type { Transaction, Tag } from '$lib/types';

	let transactions = $state<Transaction[]>([]);
	let tags = $state<Tag[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// Filter states
	let filterType = $state<'all' | 'add' | 'subtract'>('all');
	let searchQuery = $state('');
	let startDate = $state('');
	let endDate = $state('');
	let minAmount = $state('');
	let maxAmount = $state('');
	let selectedTagIds = $state<number[]>([]);
	let showAdvancedFilters = $state(false);

	onMount(async () => {
		await loadTags();
		await loadTransactions();
	});

	async function loadTags() {
		const response = await api.getTags();
		if (response.data) {
			tags = response.data;
		}
	}

	async function loadTransactions() {
		loading = true;
		error = null;

		// Build filters object
		const filters: any = {};

		if (filterType !== 'all') {
			filters.type = filterType;
		}

		if (searchQuery) {
			filters.search = searchQuery;
		}

		if (startDate) {
			filters.start_date = new Date(startDate).toISOString();
		}

		if (endDate) {
			filters.end_date = new Date(endDate + 'T23:59:59').toISOString();
		}

		if (minAmount) {
			filters.min_amount = parseInt(minAmount);
		}

		if (maxAmount) {
			filters.max_amount = parseInt(maxAmount);
		}

		if (selectedTagIds.length > 0) {
			filters.tag_ids = selectedTagIds;
		}

		const response = await api.getTransactions(50, 0, filters);
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

	function toggleTag(tagId: number) {
		if (selectedTagIds.includes(tagId)) {
			selectedTagIds = selectedTagIds.filter(id => id !== tagId);
		} else {
			selectedTagIds = [...selectedTagIds, tagId];
		}
	}

	function clearFilters() {
		filterType = 'all';
		searchQuery = '';
		startDate = '';
		endDate = '';
		minAmount = '';
		maxAmount = '';
		selectedTagIds = [];
		loadTransactions();
	}

	// Reload when filters change
	$effect(() => {
		// Watch for filter changes
		void filterType;
		void searchQuery;
		void startDate;
		void endDate;
		void minAmount;
		void maxAmount;
		void selectedTagIds.length;

		// Debounce to avoid too many API calls
		const timeoutId = setTimeout(() => {
			loadTransactions();
		}, 300);

		return () => clearTimeout(timeoutId);
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
		<div class="space-y-4">
			<!-- Basic Filters -->
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

			<!-- Advanced Filters Toggle -->
			<div class="flex justify-between items-center">
				<button
					onclick={() => (showAdvancedFilters = !showAdvancedFilters)}
					class="text-primary-600 hover:text-primary-700 text-sm font-medium"
				>
					{showAdvancedFilters ? 'Hide' : 'Show'} Advanced Filters
				</button>
				{#if startDate || endDate || minAmount || maxAmount || selectedTagIds.length > 0}
					<button onclick={clearFilters} class="text-red-600 hover:text-red-700 text-sm">
						Clear All Filters
					</button>
				{/if}
			</div>

			<!-- Advanced Filters -->
			{#if showAdvancedFilters}
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-4 border-t border-gray-200">
					<!-- Date Range -->
					<div>
						<label class="label">Start Date</label>
						<input type="date" bind:value={startDate} class="input" />
					</div>
					<div>
						<label class="label">End Date</label>
						<input type="date" bind:value={endDate} class="input" />
					</div>

					<!-- Amount Range -->
					<div>
						<label class="label">Min Amount (VND)</label>
						<input
							type="number"
							placeholder="0"
							bind:value={minAmount}
							class="input"
							min="0"
						/>
					</div>
					<div>
						<label class="label">Max Amount (VND)</label>
						<input
							type="number"
							placeholder="No limit"
							bind:value={maxAmount}
							class="input"
							min="0"
						/>
					</div>

					<!-- Tags Filter -->
					{#if tags.length > 0}
						<div class="md:col-span-2">
							<label class="label">Filter by Tags</label>
							<div class="flex flex-wrap gap-2 mt-2">
								{#each tags as tag}
									<button
										onclick={() => toggleTag(tag.id)}
										class="px-3 py-1 rounded-full text-sm transition-colors {selectedTagIds.includes(
											tag.id
										)
											? 'bg-primary-600 text-white'
											: 'bg-gray-200 text-gray-700 hover:bg-gray-300'}"
									>
										{tag.name}
									</button>
								{/each}
							</div>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	<!-- Transactions List -->
	<div class="card">
		{#if loading}
			<div class="text-center py-8">
				<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
				<p class="mt-2 text-gray-600">Loading transactions...</p>
			</div>
		{:else if transactions.length === 0}
			<p class="text-gray-500 text-center py-8">No transactions found</p>
		{:else}
			<div class="space-y-3">
				{#each transactions as transaction}
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
