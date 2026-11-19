<script lang="ts">
	import { onMount } from "svelte";
	import { page } from "$app/stores";
	import { goto } from "$app/navigation";
	import { api } from "$lib/api/client";
	import { listQuery } from "$lib/stores/transactions";
	import type { Transaction, Tag } from "$lib/types";
	import TransactionItem from "$lib/components/TransactionItem.svelte";

	const PAGE_SIZE = 50;

	let transactions = $state<Transaction[]>([]);
	let tags = $state<Tag[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let hasNext = $state(false);

	// Filters and page live in the URL, so back/reload from a transaction restores them.
	const urlParams = $page.url.searchParams;
	function fromUrl<T extends string>(key: string, allowed: readonly T[], fallback: T): T {
		return allowed.find((v) => v === urlParams.get(key)) ?? fallback;
	}

	let filterType = $state(fromUrl("type", ["all", "add", "subtract"], "all"));
	let filterStatus = $state(fromUrl("status", ["pending", "completed", "all"], "pending"));
	let searchQuery = $state(urlParams.get("q") ?? "");
	let startDate = $state(urlParams.get("from") ?? "");
	let endDate = $state(urlParams.get("to") ?? "");
	let minAmount = $state(urlParams.get("min") ?? "");
	let maxAmount = $state(urlParams.get("max") ?? "");
	let selectedTagIds = $state<number[]>(
		(urlParams.get("tags") ?? "").split(",").map(Number).filter(Boolean)
	);
	let pageNum = $state(Math.max(1, parseInt(urlParams.get("page") ?? "1") || 1));
	let showAdvancedFilters = $state(["from", "to", "min", "max", "tags"].some((k) => urlParams.get(k)));

	onMount(loadTags);

	function filterKey() {
		return JSON.stringify([
			filterType,
			filterStatus,
			searchQuery,
			startDate,
			endDate,
			minAmount,
			maxAmount,
			selectedTagIds,
		]);
	}

	function syncUrl() {
		const params = new URLSearchParams();
		if (filterType !== "all") params.set("type", filterType);
		if (filterStatus !== "pending") params.set("status", filterStatus);
		if (searchQuery) params.set("q", searchQuery);
		if (startDate) params.set("from", startDate);
		if (endDate) params.set("to", endDate);
		if (minAmount) params.set("min", minAmount);
		if (maxAmount) params.set("max", maxAmount);
		if (selectedTagIds.length) params.set("tags", selectedTagIds.join(","));
		if (pageNum > 1) params.set("page", String(pageNum));
		const qs = params.toString();
		listQuery.set(qs ? "?" + qs : "");
		goto(qs ? "?" + qs : $page.url.pathname, { replaceState: true, keepFocus: true, noScroll: true });
	}

	function goToPage(n: number) {
		pageNum = n;
		window.scrollTo({ top: 0 });
	}

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

		if (filterType !== "all") {
			filters.type = filterType;
		}

		if (filterStatus !== "all") {
			filters.completed = filterStatus === "completed";
		}

		if (searchQuery) {
			filters.search = searchQuery;
		}

		if (startDate) {
			filters.start_date = new Date(startDate).toISOString();
		}

		if (endDate) {
			filters.end_date = new Date(endDate + "T23:59:59").toISOString();
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

		// Ask for one extra row to know whether a next page exists.
		const response = await api.getTransactions(PAGE_SIZE + 1, (pageNum - 1) * PAGE_SIZE, filters);
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			if (response.data.length === 0 && pageNum > 1) {
				// The last page emptied (e.g. everything on it was completed): step back, the effect reloads.
				pageNum -= 1;
				return;
			}
			hasNext = response.data.length > PAGE_SIZE;
			transactions = response.data.slice(0, PAGE_SIZE);
		}

		loading = false;
	}

	async function completeTransactions(ids: number[]) {
		const response = await api.completeTransactions(ids);
		if (response.error) {
			error = response.error;
			return;
		}
		await loadTransactions();
	}

	function toggleTag(tagId: number) {
		if (selectedTagIds.includes(tagId)) {
			selectedTagIds = selectedTagIds.filter((id) => id !== tagId);
		} else {
			selectedTagIds = [...selectedTagIds, tagId];
		}
	}

	function clearFilters() {
		filterType = "all";
		searchQuery = "";
		startDate = "";
		endDate = "";
		minAmount = "";
		maxAmount = "";
		selectedTagIds = [];
	}

	let seenKey = filterKey(); // last filter set we saw: a change sends us back to page 1
	let loadedKey = seenKey; // last filter set we loaded: only a new one is worth debouncing

	// Reload when filters or page change
	$effect(() => {
		const key = filterKey();
		if (key !== seenKey) {
			seenKey = key;
			pageNum = 1;
		}
		void pageNum;

		// Debounce typing to avoid too many API calls; paging and the first load go straight through
		const timeoutId = setTimeout(
			() => {
				loadedKey = key;
				syncUrl();
				loadTransactions();
			},
			key === loadedKey ? 0 : 300
		);

		return () => clearTimeout(timeoutId);
	});
</script>

<div class="space-y-6">
	<div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4">
		<h1 class="text-2xl sm:text-3xl font-bold text-gray-900">Transactions</h1>
		<a href="/transactions/new" class="btn btn-primary text-sm sm:text-base">
			<svg class="w-5 h-5 inline-block mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
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
			<div class="flex flex-col gap-3">
				<div class="flex-1">
					<input
						type="text"
						placeholder="Search transactions..."
						bind:value={searchQuery}
						class="input"
					/>
				</div>
				<div class="grid grid-cols-3 gap-2">
					<button
						onclick={() => (filterType = "all")}
						class="btn text-sm sm:text-base {filterType === 'all'
							? 'btn-primary'
							: 'btn-secondary'}"
					>
						All
					</button>
					<button
						onclick={() => (filterType = "add")}
						class="btn text-sm sm:text-base {filterType === 'add'
							? 'btn-primary'
							: 'btn-secondary'}"
					>
						Income
					</button>
					<button
						onclick={() => (filterType = "subtract")}
						class="btn text-sm sm:text-base {filterType === 'subtract'
							? 'btn-primary'
							: 'btn-secondary'}"
					>
						Expense
					</button>
				</div>
			</div>

			<div class="grid grid-cols-3 gap-2">
				{#each [["pending", "Pending"], ["completed", "Completed"], ["all", "All status"]] as [value, label]}
					<button
						onclick={() => (filterStatus = value as typeof filterStatus)}
						class="btn text-sm sm:text-base {filterStatus === value ? 'btn-primary' : 'btn-secondary'}"
					>
						{label}
					</button>
				{/each}
			</div>

			<!-- Advanced Filters Toggle -->
			<div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-2">
				<button
					onclick={() => (showAdvancedFilters = !showAdvancedFilters)}
					class="text-primary-600 hover:text-primary-700 text-sm font-medium text-left"
				>
					{showAdvancedFilters ? "Hide" : "Show"} Advanced Filters
				</button>
				{#if startDate || endDate || minAmount || maxAmount || selectedTagIds.length > 0}
					<button
						onclick={clearFilters}
						class="text-red-600 hover:text-red-700 text-sm text-left sm:text-right"
					>
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
						<input type="number" placeholder="0" bind:value={minAmount} class="input" min="0" />
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
				<div
					class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"
				></div>
				<p class="mt-2 text-gray-600">Loading transactions...</p>
			</div>
		{:else if transactions.length === 0}
			<p class="text-gray-500 text-center py-8">No transactions found</p>
		{:else}
			{@const pendingIds = transactions.filter((t) => !t.completed).map((t) => t.id)}
			{#if pendingIds.length > 0}
				<div class="flex justify-end mb-3">
					<button onclick={() => completeTransactions(pendingIds)} class="btn btn-secondary text-sm">
						Mark all complete ({pendingIds.length})
					</button>
				</div>
			{/if}
			<div class="space-y-3">
				{#each transactions as transaction}
					<TransactionItem
						{transaction}
						showIcon={true}
						showDate={true}
						oncomplete={(id) => completeTransactions([id])}
					/>
				{/each}
			</div>
			{#if pageNum > 1 || hasNext}
				<div class="flex items-center justify-between mt-4">
					<button
						onclick={() => goToPage(pageNum - 1)}
						disabled={pageNum === 1}
						class="btn btn-secondary text-sm disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Previous
					</button>
					<span class="text-sm text-gray-600">Page {pageNum}</span>
					<button
						onclick={() => goToPage(pageNum + 1)}
						disabled={!hasNext}
						class="btn btn-secondary text-sm disabled:opacity-50 disabled:cursor-not-allowed"
					>
						Next
					</button>
				</div>
			{/if}
		{/if}
	</div>
</div>
