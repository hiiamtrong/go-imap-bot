<script lang="ts">
  import {
    formatCurrency,
    formatAmount,
    formatRelativeTime,
    formatDate,
  } from "$lib/utils/format";
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
  class="block p-4 border border-gray-200 rounded-lg hover:border-primary-300 hover:shadow-md transition-all"
>
  <div class="flex justify-between items-start">
    <div class="flex-1">
      <div class="flex items-center gap-3">
        {#if showIcon}
          <div
            class="w-10 h-10 rounded-full flex items-center justify-center
						{transaction.type === 'add' ? 'bg-green-100' : 'bg-red-100'}"
          >
            {#if transaction.type === "add"}
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
        {/if}

        <div class="flex-1">
          <div class="flex items-center gap-2">
            <p class="font-medium text-gray-900">{transaction.description}</p>
            {#if transaction.completed}
              <span
                class="inline-flex items-center px-2 py-0.5 bg-green-100 text-green-700 text-xs font-medium rounded-full"
              >
                <svg
                  class="w-3 h-3 mr-0.5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
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
          <p class="text-sm text-gray-500 mt-1">
            {formatRelativeTime(transaction.timestamp)}
            {#if showDate}
              • {formatDate(transaction.timestamp)}
            {/if}
          </p>

          {#if transaction.tags && transaction.tags.length > 0}
            <div class="flex gap-2 mt-2" class:ml-13={showIcon}>
              {#each transaction.tags as tag}
                <span
                  class="px-2 py-1 bg-primary-100 text-primary-700 text-xs rounded-full"
                >
                  {tag.name}
                </span>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>

    <div class="text-right ml-4">
      <p
        class="{showDate
          ? 'text-xl'
          : 'text-lg'} font-bold {getTransactionTypeColor(transaction.type)}"
      >
        {formatAmount(transaction.amount, transaction.type)}
      </p>
      <p class="text-sm text-gray-500 mt-1">
        Balance: {formatCurrency(transaction.balance)}
      </p>
      {#if transaction.splits && transaction.splits.length > 0}
        <div class="mt-2">
          {#if showDate}
            <span
              class="px-2 py-1 text-xs rounded-full
							{transaction.splits.some((s) => !s.completed)
                ? 'bg-orange-100 text-orange-700'
                : 'bg-green-100 text-green-700'}"
            >
              {transaction.splits.filter((s) => !s.completed).length === 0
                ? "All paid"
                : `${transaction.splits.filter((s) => !s.completed).length} pending splits`}
            </span>
          {:else}
            <p class="text-xs text-gray-500">
              {transaction.splits.filter((s) => !s.completed).length} pending splits
            </p>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</a>
