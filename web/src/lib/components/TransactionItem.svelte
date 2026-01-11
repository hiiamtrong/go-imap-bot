<script lang="ts">
	import { formatCurrency, formatAmount, formatRelativeTime, formatDate } from "$lib/utils/format";
	import type { Transaction } from "$lib/types";

	interface Props {
		transaction: Transaction;
		showIcon?: boolean;
		showDate?: boolean;
	}

	let { transaction, showIcon = false, showDate = false }: Props = $props();

	function getTransactionTypeColor(type: "add" | "subtract"): string {
		return type === "add" ? "text-green-600" : "text-red-600";
	}
</script>

<a
	href="/transactions/{transaction.id}"
	class="block p-3 sm:p-4 border border-gray-200 rounded-lg hover:border-primary-300 hover:shadow-md transition-all"
>
	<div class="flex flex-col sm:flex-row sm:justify-between gap-3 sm:gap-4">
		<div class="flex-1 min-w-0">
			<div class="flex items-start gap-3">
				{#if showIcon}
					<div
						class="w-8 h-8 sm:w-10 sm:h-10 rounded-full flex items-center justify-center flex-shrink-0
						{transaction.type === 'add' ? 'bg-green-100' : 'bg-red-100'}"
					>
						{#if transaction.type === "add"}
							<svg
								class="w-4 h-4 sm:w-5 sm:h-5 text-green-600"
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
								class="w-4 h-4 sm:w-5 sm:h-5 text-red-600"
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
				{/if}

				<div class="flex-1 min-w-0">
					<div class="flex items-center gap-2 flex-wrap">
						<p class="font-medium text-gray-900 text-sm sm:text-base truncate">
							{transaction.description}
						</p>
						{#if transaction.completed}
							<span
								class="inline-flex items-center px-2 py-0.5 bg-green-100 text-green-700 text-xs font-medium rounded-full flex-shrink-0"
							>
								<svg class="w-3 h-3 mr-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M5 13l4 4L19 7"
									/>
								</svg>
								Completed
							</span>
						{/if}
					</div>
					<p class="text-xs sm:text-sm text-gray-500 mt-1">
						{formatRelativeTime(transaction.timestamp)}
						{#if showDate}
							<span class="hidden sm:inline">• {formatDate(transaction.timestamp)}</span>
						{/if}
					</p>

					{#if transaction.tags && transaction.tags.length > 0}
						<div class="flex flex-wrap gap-1.5 sm:gap-2 mt-2">
							{#each transaction.tags as tag}
								<span
									class="px-2 py-0.5 sm:py-1 bg-primary-100 text-primary-700 text-xs rounded-full"
								>
									{tag.name}
								</span>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		</div>

		<div
			class="sm:text-right flex sm:flex-col justify-between sm:justify-start items-end sm:items-end flex-shrink-0"
		>
			<div>
				<p
					class="{showDate
						? 'text-lg sm:text-xl'
						: 'text-base sm:text-lg'} font-bold {getTransactionTypeColor(transaction.type)}"
				>
					{formatAmount(transaction.amount, transaction.type, transaction.currency || "VND")}
				</p>
				<p class="text-xs sm:text-sm text-gray-500 mt-0.5 sm:mt-1">
					Balance: {formatCurrency(transaction.balance, transaction.currency || "VND")}
				</p>
			</div>
			{#if transaction.splits && transaction.splits.length > 0}
				<div class="sm:mt-2">
					{#if showDate}
						<span
							class="px-2 py-1 text-xs rounded-full whitespace-nowrap
							{transaction.splits.some((s) => !s.completed)
								? 'bg-orange-100 text-orange-700'
								: 'bg-green-100 text-green-700'}"
						>
							{transaction.splits.filter((s) => !s.completed).length === 0
								? "All paid"
								: `${transaction.splits.filter((s) => !s.completed).length} pending`}
						</span>
					{:else}
						<p class="text-xs text-gray-500 whitespace-nowrap">
							{transaction.splits.filter((s) => !s.completed).length} pending
						</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</a>
