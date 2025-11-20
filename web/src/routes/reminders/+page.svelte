<script lang="ts">
	import { onMount } from "svelte";
	import { api } from "$lib/api/client";
	import { formatCurrency } from "$lib/utils/format";
	import type { UserSplitSummary, TransactionSplit } from "$lib/types";

	let userSummaries = $state<UserSplitSummary[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let selectedUsers = $state<Set<number>>(new Set());
	let expandedUsers = $state<Set<number>>(new Set());
	let angryMode = $state(false);
	let sending = $state(false);
	let completingUsers = $state<Set<number>>(new Set());
	let completingSplits = $state<Set<number>>(new Set());

	// Edit modal state
	let editingSplit = $state<TransactionSplit | null>(null);
	let editReason = $state("");
	let editAmount = $state(""); // Raw numeric value
	let displayEditAmount = $state(""); // Formatted display value
	let saving = $state(false);

	const totalSummary = $derived(() => {
		return {
			userCount: userSummaries.length,
			billCount: userSummaries.reduce((sum, u) => sum + u.bill_count, 0),
			totalAmount: userSummaries.reduce((sum, u) => sum + u.total_amount, 0),
		};
	});

	onMount(async () => {
		await loadPendingTransactions();
	});

	async function loadPendingTransactions() {
		loading = true;
		error = null;

		const response = await api.getPendingSplits();
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			userSummaries = response.data.sort((a, b) => a.user.name.localeCompare(b.user.name));
		}

		loading = false;
	}

	function toggleUser(userId: number) {
		const newSet = new Set(selectedUsers);
		if (newSet.has(userId)) {
			newSet.delete(userId);
		} else {
			newSet.add(userId);
		}
		selectedUsers = newSet;
	}

	function selectAll() {
		selectedUsers = new Set(userSummaries.map((u) => u.user.id));
	}

	function clearSelection() {
		selectedUsers = new Set();
	}

	async function sendReminders() {
		const splitIds: number[] = [];
		userSummaries.forEach((summary) => {
			if (selectedUsers.has(summary.user.id)) {
				summary.splits.forEach((split) => splitIds.push(split.id));
			}
		});

		if (splitIds.length === 0) {
			error = "Please select at least one user to send reminders";
			return;
		}

		sending = true;
		error = null;

		const response = await api.sendReminder({
			split_ids: splitIds,
			angry: angryMode,
		});

		if (response.error) {
			error = response.error;
		} else {
			alert(`Successfully sent ${splitIds.length} reminder(s) to ${selectedUsers.size} user(s)!`);
			selectedUsers = new Set();
			await loadPendingTransactions();
		}

		sending = false;
	}

	function getStatusIcon(summary: UserSplitSummary): string {
		return summary.user.whitelist ? "🟢" : "🔴";
	}

	function toggleExpand(userId: number) {
		const newSet = new Set(expandedUsers);
		if (newSet.has(userId)) {
			newSet.delete(userId);
		} else {
			newSet.add(userId);
		}
		expandedUsers = newSet;
	}

	async function completeUserBills(userId: number, userName: string) {
		if (!confirm(`Xác nhận đã thanh toán TẤT CẢ các bill của ${userName}?`)) return;

		completingUsers.add(userId);
		completingUsers = new Set(completingUsers);

		// Get any split ID from this user to trigger completion
		const userSummary = userSummaries.find((s) => s.user.id === userId);
		if (!userSummary || userSummary.splits.length === 0) {
			error = "Không tìm thấy bill nào để hoàn thành";
			completingUsers.delete(userId);
			completingUsers = new Set(completingUsers);
			return;
		}

		const response = await api.completeSplit(userSummary.splits[0].id);
		if (response.error) {
			error = response.error;
		} else {
			// Reload data
			await loadPendingTransactions();
		}

		completingUsers.delete(userId);
		completingUsers = new Set(completingUsers);
	}

	async function completeSingleSplit(splitId: number) {
		if (!confirm("Xác nhận đã thanh toán bill này?")) return;

		completingSplits.add(splitId);
		completingSplits = new Set(completingSplits);

		const response = await api.completeSingleSplit(splitId);
		if (response.error) {
			error = response.error;
		} else {
			await loadPendingTransactions();
		}

		completingSplits.delete(splitId);
		completingSplits = new Set(completingSplits);
	}

	function openEditModal(split: TransactionSplit) {
		editingSplit = split;
		editReason = split.reason || "";
		const isNegative = split.amount < 0;
		editAmount = split.amount.toString(); // Raw numeric value
		// Format for display
		const formatted = formatCurrency(Math.abs(split.amount)).replace("₫", "").trim();
		displayEditAmount = isNegative ? "-" + formatted : formatted;
	}

	function closeEditModal() {
		editingSplit = null;
		editReason = "";
		editAmount = "";
		displayEditAmount = "";
	}

	function handleAmountKeyDown(e: KeyboardEvent) {
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
		// Allow: minus/dash key (189 for dash, 109 for numpad minus)
		if (e.keyCode === 189 || e.keyCode === 109) {
			return;
		}
		// Ensure that it is a number and stop the keypress if not
		if ((e.shiftKey || e.keyCode < 48 || e.keyCode > 57) && (e.keyCode < 96 || e.keyCode > 105)) {
			e.preventDefault();
		}
	}

	function handleEditAmountInput(e: Event) {
		const input = e.target as HTMLInputElement;
		// Allow digits and minus sign at the beginning
		let value = input.value;
		const isNegative = value.startsWith("-");
		// Remove everything except digits
		value = value.replace(/[^\d]/g, "");

		if (value === "") {
			editAmount = isNegative ? "-" : "";
			displayEditAmount = isNegative ? "-" : "";
			return;
		}

		// Store the raw numeric value (with sign)
		editAmount = isNegative ? "-" + value : value;

		// Format for display with thousand separators
		const numValue = parseInt(value, 10);
		const formatted = formatCurrency(numValue).replace("₫", "").trim();
		displayEditAmount = isNegative ? "-" + formatted : formatted;

		// Update cursor position to the end after formatting
		setTimeout(() => {
			input.selectionStart = input.selectionEnd = input.value.length;
		}, 0);
	}

	async function saveEdit() {
		if (!editingSplit) return;

		saving = true;
		error = null;

		const amount = parseInt(editAmount, 10); // Raw numeric value
		if (isNaN(amount) || amount === 0) {
			error = "Số tiền không hợp lệ";
			saving = false;
			return;
		}

		const response = await api.updateSplit(editingSplit.id, {
			amount,
			reason: editReason,
		});

		if (response.error) {
			error = response.error;
		} else {
			closeEditModal();
			await loadPendingTransactions();
		}

		saving = false;
	}
</script>

<div class="space-y-6">
	<div class="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4">
		<h1 class="text-2xl sm:text-3xl font-bold text-gray-900">📧 Danh sách nhắc nhở</h1>
		<div class="flex gap-2">
			<button
				onclick={selectAll}
				disabled={loading || userSummaries.length === 0}
				class="btn btn-secondary disabled:opacity-50 text-sm sm:text-base"
			>
				Select All
			</button>
			{#if selectedUsers.size > 0}
				<button onclick={clearSelection} class="btn btn-secondary text-sm sm:text-base">
					Clear
				</button>
			{/if}
		</div>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	<!-- Reminder Options -->
	<div class="card">
		<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
			<div class="flex items-center gap-4">
				<label class="flex items-center cursor-pointer">
					<input type="checkbox" bind:checked={angryMode} class="mr-2" />
					<span class="text-sm sm:text-base text-gray-700"> Use "Angry" reminder template </span>
				</label>
			</div>
			{#if selectedUsers.size > 0}
				<button
					onclick={sendReminders}
					disabled={sending}
					class="btn btn-primary disabled:opacity-50 text-sm sm:text-base w-full sm:w-auto"
				>
					{sending
						? "Sending..."
						: `Send Reminders to ${selectedUsers.size} User${selectedUsers.size > 1 ? "s" : ""}`}
				</button>
			{/if}
		</div>
	</div>

	<!-- User List -->
	<div class="card">
		{#if loading}
			<div class="text-center py-8">
				<div
					class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"
				></div>
				<p class="mt-2 text-gray-600">Loading pending payments...</p>
			</div>
		{:else if userSummaries.length === 0}
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
			<div class="space-y-3">
				{#each userSummaries as summary}
					<div class="border border-gray-200 rounded-lg overflow-hidden">
						<div class="p-4 flex flex-col sm:flex-row sm:items-start gap-4">
							<label class="flex items-start gap-3 flex-1 cursor-pointer min-w-0">
								<input
									type="checkbox"
									checked={selectedUsers.has(summary.user.id)}
									onchange={() => toggleUser(summary.user.id)}
									class="mt-1 flex-shrink-0"
								/>
								<div class="flex-1 min-w-0">
									<div class="flex items-center gap-2 mb-1">
										<h3 class="font-bold text-gray-900 truncate">{summary.user.name}</h3>
										<span class="text-xl flex-shrink-0">{getStatusIcon(summary)}</span>
									</div>
									<p class="text-sm text-gray-600 mb-1 truncate">
										Email: {summary.user.email}
									</p>
									<div class="flex flex-wrap gap-2 sm:gap-4 text-sm text-gray-600">
										<span>Số lượng bill: <strong>{summary.bill_count}</strong></span>
										<span
											>Tổng tiền: <strong class="text-orange-600"
												>{formatCurrency(summary.total_amount)}</strong
											></span
										>
									</div>
									{#if summary.user.whitelist}
										<span
											class="inline-block px-2 py-1 bg-green-100 text-green-700 text-xs rounded-full mt-2"
										>
											Whitelisted - Sẽ không gửi reminder
										</span>
									{/if}
								</div>
							</label>
							<div class="flex flex-col sm:flex-row gap-2 sm:flex-shrink-0">
								<button
									onclick={() => completeUserBills(summary.user.id, summary.user.name)}
									disabled={completingUsers.has(summary.user.id)}
									class="btn btn-primary text-sm py-2 px-3 disabled:opacity-50 whitespace-nowrap"
								>
									{completingUsers.has(summary.user.id) ? "Đang xử lý..." : "Đánh dấu đã xong"}
								</button>
								<button
									type="button"
									onclick={() => toggleExpand(summary.user.id)}
									class="p-2 hover:bg-gray-100 rounded transition-colors border border-gray-300 flex items-center justify-center gap-2"
									aria-label="Toggle bill details"
								>
									<span class="text-sm text-gray-700 sm:hidden">Chi tiết</span>
									<svg
										class="w-5 h-5 text-gray-600 transition-transform"
										class:rotate-180={expandedUsers.has(summary.user.id)}
										fill="none"
										stroke="currentColor"
										viewBox="0 0 24 24"
									>
										<path
											stroke-linecap="round"
											stroke-linejoin="round"
											stroke-width="2"
											d="M19 9l-7 7-7-7"
										/>
									</svg>
								</button>
							</div>
						</div>

						{#if expandedUsers.has(summary.user.id)}
							<div class="border-t border-gray-200 bg-gray-50 p-4">
								<h4 class="text-sm font-semibold text-gray-700 mb-3">Chi tiết các bill:</h4>
								<div class="space-y-2">
									{#each summary.splits as split}
										<div
											class="flex items-center justify-between p-3 bg-white rounded border border-gray-200"
										>
											<div class="flex-1">
												{#if split.reason}
													<p class="text-sm font-medium text-gray-900">
														{split.reason}
													</p>
												{/if}
												<p class="text-sm font-bold text-orange-600">
													{formatCurrency(split.amount)}
												</p>
												<p class="text-xs text-gray-500 mt-1">
													{new Date(split.created_at).toLocaleString("vi-VN")}
												</p>
											</div>
											<div class="flex items-center gap-2 ml-3">
												<button
													onclick={() => openEditModal(split)}
													class="p-1.5 text-blue-600 hover:bg-blue-50 rounded transition-colors"
													title="Chỉnh sửa"
												>
													<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
														<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
													</svg>
												</button>
												<button
													onclick={() => completeSingleSplit(split.id)}
													disabled={completingSplits.has(split.id)}
													class="p-1.5 text-green-600 hover:bg-green-50 rounded transition-colors disabled:opacity-50"
													title="Đánh dấu đã xong"
												>
													{#if completingSplits.has(split.id)}
														<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
															<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
															<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
														</svg>
													{:else}
														<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
															<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
														</svg>
													{/if}
												</button>
											</div>
										</div>
									{/each}
								</div>
							</div>
						{/if}
					</div>
				{/each}
			</div>

			<!-- Summary -->
			<div class="mt-6 pt-6 border-t border-gray-200">
				<h3 class="font-bold text-gray-900 mb-3">Tổng cộng:</h3>
				<div class="grid grid-cols-1 sm:grid-cols-3 gap-3 sm:gap-4 text-center">
					<div class="p-3 bg-blue-50 rounded-lg">
						<p class="text-xs sm:text-sm text-gray-600">Số người</p>
						<p class="text-xl sm:text-2xl font-bold text-blue-600">
							{totalSummary().userCount}
						</p>
					</div>
					<div class="p-3 bg-orange-50 rounded-lg">
						<p class="text-xs sm:text-sm text-gray-600">Số bill</p>
						<p class="text-xl sm:text-2xl font-bold text-orange-600">
							{totalSummary().billCount}
						</p>
					</div>
					<div class="p-3 bg-green-50 rounded-lg">
						<p class="text-xs sm:text-sm text-gray-600">Tổng tiền</p>
						<p class="text-xl sm:text-2xl font-bold text-green-600">
							{formatCurrency(totalSummary().totalAmount)}
						</p>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

<!-- Edit Modal -->
{#if editingSplit}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
		<div class="bg-white rounded-lg shadow-xl max-w-md w-full p-6">
			<h3 class="text-lg font-bold text-gray-900 mb-4">Chỉnh sửa bill</h3>

			<div class="space-y-4">
				<div>
					<label for="edit-reason" class="block text-sm font-medium text-gray-700 mb-1">
						Lý do
					</label>
					<input
						id="edit-reason"
						type="text"
						bind:value={editReason}
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
						placeholder="Nhập lý do..."
					/>
				</div>

				<div>
					<label for="edit-amount" class="block text-sm font-medium text-gray-700 mb-1">
						Số tiền (VNĐ)
					</label>
					<input
						id="edit-amount"
						type="text"
						value={displayEditAmount}
						onkeydown={handleAmountKeyDown}
						oninput={handleEditAmountInput}
						class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
						placeholder="Nhập số tiền..."
					/>
				</div>
			</div>

			<div class="flex justify-end gap-3 mt-6">
				<button
					onclick={closeEditModal}
					class="px-4 py-2 text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
				>
					Hủy
				</button>
				<button
					onclick={saveEdit}
					disabled={saving}
					class="px-4 py-2 text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors disabled:opacity-50"
				>
					{saving ? "Đang lưu..." : "Lưu"}
				</button>
			</div>
		</div>
	</div>
{/if}
