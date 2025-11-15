<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "$lib/api/client";
  import { formatCurrency } from "$lib/utils/format";
  import type { Transaction, User } from "$lib/types";

  interface Props {
    transaction: Transaction;
    onClose: () => void;
  }

  let { transaction, onClose }: Props = $props();

  let users = $state<User[]>([]);
  let selectedUsers = $state<Set<number>>(new Set());
  let splitMode = $state<"custom" | "equal">("equal");
  let customAmounts = $state<Record<number, string>>({});
  let displayAmounts = $state<Record<number, string>>({});
  let reasons = $state<Record<number, string>>({});
  let loading = $state(false);
  let error = $state<string | null>(null);
  let searchQuery = $state("");
  let globalReason = $state("");
  let showCreateUser = $state(false);
  let newUserName = $state("");
  let newUserEmail = $state("");
  let creatingUser = $state(false);

  onMount(async () => {
    await loadUsers();
  });

  async function loadUsers() {
    const response = await api.getUsers();
    if (response.data) {
      users = response.data;
    }
  }

  async function handleCreateUser() {
    if (!newUserName.trim() || !newUserEmail.trim()) {
      error = "Please enter both name and email";
      return;
    }

    creatingUser = true;
    error = null;

    const response = await api.createUser({
      name: newUserName.trim(),
      email: newUserEmail.trim(),
      whitelist: false,
    });

    if (response.error) {
      error = response.error;
    } else if (response.data) {
      // Reload users list
      await loadUsers();
      // Auto-select the newly created user
      selectedUsers.add(response.data.id);
      selectedUsers = new Set(selectedUsers);
      // Clear form and hide
      newUserName = "";
      newUserEmail = "";
      showCreateUser = false;
    }

    creatingUser = false;
  }

  // Get selected users with their details
  let selectedUsersList = $derived(
    users.filter((user) => selectedUsers.has(user.id))
  );

  // Filter users based on search query
  let filteredUsers = $derived(
    users.filter(
      (user) =>
        user.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        user.email.toLowerCase().includes(searchQuery.toLowerCase())
    )
  );

  function toggleUser(userId: number) {
    const newSet = new Set(selectedUsers);
    if (newSet.has(userId)) {
      newSet.delete(userId);
      delete customAmounts[userId];
      delete displayAmounts[userId];
      delete reasons[userId];
    } else {
      newSet.add(userId);
    }
    selectedUsers = newSet;
  }

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
    if (
      (e.shiftKey || e.keyCode < 48 || e.keyCode > 57) &&
      (e.keyCode < 96 || e.keyCode > 105)
    ) {
      e.preventDefault();
    }
  }

  function handleAmountInput(e: Event, userId: number) {
    const input = e.target as HTMLInputElement;
    // Only allow digits (no decimal separator)
    const value = input.value.replace(/[^\d]/g, "");

    if (value === "") {
      customAmounts[userId] = "";
      displayAmounts[userId] = "";
      return;
    }

    // Store the raw numeric value
    customAmounts[userId] = value;

    // Format for display with thousand separators
    const numValue = parseInt(value, 10);
    displayAmounts[userId] = formatCurrency(numValue).replace("₫", "").trim();

    // Update cursor position to the end after formatting
    setTimeout(() => {
      input.selectionStart = input.selectionEnd = input.value.length;
    }, 0);
  }

  function calculateEqualSplit(): number {
    const count = selectedUsers.size;
    if (count === 0) return 0;
    return Math.abs(transaction.amount) / count;
  }

  function calculateTotal(): number {
    if (splitMode === "equal") {
      return calculateEqualSplit() * selectedUsers.size;
    }

    let total = 0;
    selectedUsers.forEach((userId) => {
      const amount = parseFloat(customAmounts[userId] || "0");
      total += amount;
    });
    return total;
  }

  function calculateRemaining(): number {
    const transactionAmount = Math.abs(transaction.amount);
    const total = calculateTotal();
    return transactionAmount - total;
  }

  function isValidSplit(): boolean {
    if (selectedUsers.size === 0) return false;
    const remaining = calculateRemaining();
    return Math.abs(remaining) < 0.01;
  }

  async function handleSubmit() {
    if (!isValidSplit()) {
      error = "Split amounts must equal transaction amount";
      return;
    }

    loading = true;
    error = null;

    // Use global reason if provided, otherwise use individual reasons, or fall back to transaction description
    const defaultReason = globalReason || transaction.description;

    const splits = Array.from(selectedUsers).map((userId) => ({
      user_id: userId,
      amount:
        splitMode === "equal"
          ? calculateEqualSplit()
          : parseFloat(customAmounts[userId] || "0"),
      reason: reasons[userId] || defaultReason,
    }));

    const response = await api.createBillSplit({
      transaction_id: transaction.id,
      users: splits,
    });

    if (response.error) {
      error = response.error;
    } else {
      onClose();
    }

    loading = false;
  }
</script>

<div
  class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50"
>
  <div
    class="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto"
  >
    <div class="p-6">
      <div class="mb-6">
        <div class="flex justify-between items-center">
          <div>
            <h2 class="text-2xl font-bold text-gray-900">Split Bill</h2>
            {#if selectedUsers.size > 0}
              <p class="text-sm text-gray-600 mt-1">
                {selectedUsers.size} user{selectedUsers.size !== 1 ? "s" : ""} selected
                {#if splitMode === "equal"}
                  • {formatCurrency(calculateEqualSplit())} each
                {:else}
                  • {formatCurrency(calculateTotal())} total
                {/if}
              </p>
            {/if}
          </div>
          <button
            onclick={onClose}
            class="text-gray-500 hover:text-gray-700"
            aria-label="Close modal"
          >
            <svg
              class="w-6 h-6"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
      </div>

      {#if error}
        <div
          class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-4"
        >
          {error}
        </div>
      {/if}

      <div class="mb-6">
        <p class="text-gray-600">Transaction Amount:</p>
        <p class="text-2xl font-bold text-gray-900">
          {formatCurrency(Math.abs(transaction.amount))}
        </p>
      </div>

      <!-- Split Mode -->
      <div class="mb-6">
        <div class="label mb-2">Split Mode</div>
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

      <!-- Global Reason -->
      <div class="mb-6">
        <label for="globalReason" class="label mb-2">
          Reason for Split (Optional)
        </label>
        <input
          id="globalReason"
          type="text"
          placeholder={`Leave empty to use: "${transaction.description}"`}
          bind:value={globalReason}
          class="input w-full"
        />
        <p class="text-xs text-gray-500 mt-1">
          This reason will be applied to all users unless they have individual
          reasons below.
        </p>
      </div>

      <!-- Selected Users Section -->
      {#if selectedUsersList.length > 0}
        <div class="mb-6">
          <div class="label mb-3">Selected Users ({selectedUsersList.length})</div>
          <div class="space-y-3 max-h-60 overflow-y-auto border border-gray-200 rounded-lg p-3 bg-blue-50">
            {#each selectedUsersList as user}
              <div class="bg-white rounded-lg p-3 space-y-2">
                <div class="flex items-start justify-between">
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2">
                      <p class="font-medium text-gray-900 truncate">{user.name}</p>
                      <button
                        onclick={() => toggleUser(user.id)}
                        class="text-red-500 hover:text-red-700 flex-shrink-0"
                        aria-label="Remove user"
                      >
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    </div>
                    <p class="text-xs text-gray-500 truncate">{user.email}</p>
                  </div>
                  <div class="ml-3 flex-shrink-0">
                    {#if splitMode === "equal"}
                      <p class="text-sm font-bold text-primary-600">
                        {formatCurrency(calculateEqualSplit())}
                      </p>
                    {:else}
                      <div class="relative">
                        <input
                          type="text"
                          inputmode="numeric"
                          placeholder="Amount"
                          value={displayAmounts[user.id] || ""}
                          oninput={(e) => handleAmountInput(e, user.id)}
                          onkeydown={handleKeyDown}
                          class="input w-28 text-sm pr-6"
                        />
                        <span
                          class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 text-xs pointer-events-none"
                        >
                          ₫
                        </span>
                      </div>
                    {/if}
                  </div>
                </div>
                <input
                  type="text"
                  placeholder="Reason (optional)"
                  bind:value={reasons[user.id]}
                  class="input text-sm w-full"
                />
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Add Users Section -->
      <div class="mb-6">
        <div class="flex justify-between items-center mb-3">
          <div class="label">Add Users to Split</div>
          <button
            onclick={() => (showCreateUser = !showCreateUser)}
            class="text-sm text-primary-600 hover:text-primary-700 font-medium"
          >
            {showCreateUser ? "Cancel" : "+ Create New User"}
          </button>
        </div>

        <!-- Create User Form -->
        {#if showCreateUser}
          <div class="mb-4 p-4 bg-gray-50 rounded-lg border border-gray-200">
            <h4 class="font-medium text-gray-900 mb-3">Create New User</h4>
            <div class="space-y-3">
              <div>
                <label class="label text-sm mb-1">Name</label>
                <input
                  type="text"
                  placeholder="Enter name"
                  bind:value={newUserName}
                  class="input w-full"
                />
              </div>
              <div>
                <label class="label text-sm mb-1">Email</label>
                <input
                  type="email"
                  placeholder="Enter email"
                  bind:value={newUserEmail}
                  class="input w-full"
                />
              </div>
              <button
                onclick={handleCreateUser}
                disabled={creatingUser || !newUserName.trim() || !newUserEmail.trim()}
                class="btn btn-primary w-full text-sm disabled:opacity-50"
              >
                {creatingUser ? "Creating..." : "Create and Add to Split"}
              </button>
            </div>
          </div>
        {/if}

        <!-- Search and Select Users -->
        <div class="mb-3">
          <input
            type="text"
            placeholder="Search users by name or email..."
            bind:value={searchQuery}
            class="input w-full"
          />
        </div>

        <div class="space-y-2 max-h-60 overflow-y-auto">
          {#if filteredUsers.length === 0}
            <p class="text-gray-500 text-center py-4 text-sm">No users found</p>
          {:else}
            {#each filteredUsers as user}
              <label
                class="flex items-center justify-between p-3 bg-gray-50 rounded-lg hover:bg-gray-100 cursor-pointer transition-colors"
              >
                <div class="flex items-center flex-1 min-w-0">
                  <input
                    type="checkbox"
                    checked={selectedUsers.has(user.id)}
                    onchange={() => toggleUser(user.id)}
                    class="mr-3 flex-shrink-0"
                  />
                  <div class="min-w-0 flex-1">
                    <p class="font-medium text-gray-900 text-sm truncate">{user.name}</p>
                    <p class="text-xs text-gray-500 truncate">{user.email}</p>
                  </div>
                </div>
                {#if selectedUsers.has(user.id)}
                  <span class="ml-2 text-xs text-primary-600 font-medium flex-shrink-0">✓ Selected</span>
                {/if}
              </label>
            {/each}
          {/if}
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
          <div class="flex justify-between mb-2">
            <span>Transaction Amount:</span>
            <span class="font-medium"
              >{formatCurrency(Math.abs(transaction.amount))}</span
            >
          </div>
          <div class="flex justify-between">
            <span>Remaining:</span>
            <span class="font-medium {Math.abs(calculateRemaining()) < 0.01 ? 'text-green-600' : calculateRemaining() > 0 ? 'text-orange-600' : 'text-red-600'}">
              {formatCurrency(Math.abs(calculateRemaining()))}
              {#if Math.abs(calculateRemaining()) < 0.01}
                <span class="text-xs">(Perfect!)</span>
              {:else if calculateRemaining() > 0}
                <span class="text-xs">(need more)</span>
              {:else}
                <span class="text-xs">(over allocated)</span>
              {/if}
            </span>
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
          {loading ? "Creating..." : "Create Split"}
        </button>
        <button onclick={onClose} class="btn btn-secondary">Cancel</button>
      </div>
    </div>
  </div>
</div>
