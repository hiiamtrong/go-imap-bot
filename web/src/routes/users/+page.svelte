<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { User } from '$lib/types';

	let users = $state<User[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let showAddModal = $state(false);
	let editingUser = $state<User | null>(null);

	// Filter states
	let searchQuery = $state('');
	let whitelistFilter = $state<'all' | 'whitelisted' | 'not-whitelisted'>('all');

	let formData = $state({
		name: '',
		email: '',
		whitelist: false
	});

	onMount(async () => {
		await loadUsers();
	});

	async function loadUsers() {
		loading = true;
		error = null;

		// Build filters object
		const filters: any = {};

		if (searchQuery) {
			filters.search = searchQuery;
		}

		if (whitelistFilter !== 'all') {
			filters.whitelist = whitelistFilter === 'whitelisted';
		}

		const response = await api.getUsers(filters);
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			users = response.data;
		}

		loading = false;
	}

	// Reload when filters change
	$effect(() => {
		void searchQuery;
		void whitelistFilter;

		const timeoutId = setTimeout(() => {
			loadUsers();
		}, 300);

		return () => clearTimeout(timeoutId);
	});

	function openAddModal() {
		editingUser = null;
		formData = { name: '', email: '', whitelist: false };
		showAddModal = true;
	}

	function openEditModal(user: User) {
		editingUser = user;
		formData = {
			name: user.name,
			email: user.email,
			whitelist: user.whitelist
		};
		showAddModal = true;
	}

	async function handleSubmit() {
		error = null;

		if (editingUser) {
			const response = await api.updateUser(editingUser.id, formData);
			if (response.error) {
				error = response.error;
				return;
			}
		} else {
			const response = await api.createUser(formData);
			if (response.error) {
				error = response.error;
				return;
			}
		}

		showAddModal = false;
		await loadUsers();
	}

	async function handleDelete(id: number) {
		if (!confirm('Are you sure you want to delete this user?')) return;

		const response = await api.deleteUser(id);
		if (response.error) {
			error = response.error;
		} else {
			await loadUsers();
		}
	}
</script>

<div class="space-y-6">
	<div class="flex justify-between items-center">
		<h1 class="text-3xl font-bold text-gray-900">Users</h1>
		<button onclick={openAddModal} class="btn btn-primary">
			<svg class="w-5 h-5 inline-block mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M12 4v16m8-8H4"
				/>
			</svg>
			Add User
		</button>
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
					placeholder="Search by name or email..."
					bind:value={searchQuery}
					class="input"
				/>
			</div>
			<div class="flex gap-2">
				<button
					onclick={() => (whitelistFilter = 'all')}
					class="btn {whitelistFilter === 'all' ? 'btn-primary' : 'btn-secondary'}"
				>
					All
				</button>
				<button
					onclick={() => (whitelistFilter = 'whitelisted')}
					class="btn {whitelistFilter === 'whitelisted' ? 'btn-primary' : 'btn-secondary'}"
				>
					Whitelisted
				</button>
				<button
					onclick={() => (whitelistFilter = 'not-whitelisted')}
					class="btn {whitelistFilter === 'not-whitelisted' ? 'btn-primary' : 'btn-secondary'}"
				>
					Not Whitelisted
				</button>
			</div>
		</div>
	</div>

	<div class="card">
		{#if loading}
			<div class="text-center py-8">
				<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
				<p class="mt-2 text-gray-600">Loading users...</p>
			</div>
		{:else if users.length === 0}
			<p class="text-gray-500 text-center py-8">No users yet</p>
		{:else}
			<div class="space-y-3">
				{#each users as user}
					<div class="flex items-center justify-between p-4 bg-gray-50 rounded-lg">
						<div class="flex items-center gap-4 flex-1">
							<div
								class="w-12 h-12 rounded-full bg-primary-100 flex items-center justify-center text-primary-700 font-bold text-lg"
							>
								{user.name.charAt(0).toUpperCase()}
							</div>
							<div>
								<p class="font-medium text-gray-900">{user.name}</p>
								<p class="text-sm text-gray-500">{user.email}</p>
								{#if user.whitelist}
									<span class="inline-block px-2 py-1 bg-green-100 text-green-700 text-xs rounded-full mt-1">
										Whitelisted
									</span>
								{/if}
							</div>
						</div>
						<div class="flex gap-2">
							<button
								onclick={() => openEditModal(user)}
								class="px-3 py-1 text-primary-600 hover:text-primary-700"
							>
								<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="2"
										d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
									/>
								</svg>
							</button>
							<button
								onclick={() => handleDelete(user.id)}
								class="px-3 py-1 text-red-600 hover:text-red-700"
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
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<!-- Add/Edit User Modal -->
{#if showAddModal}
	<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
		<div class="bg-white rounded-lg shadow-xl max-w-md w-full">
			<div class="p-6">
				<div class="flex justify-between items-center mb-6">
					<h2 class="text-2xl font-bold text-gray-900">
						{editingUser ? 'Edit User' : 'Add User'}
					</h2>
					<button
						onclick={() => (showAddModal = false)}
						class="text-gray-500 hover:text-gray-700"
					>
						<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M6 18L18 6M6 6l12 12"
							/>
						</svg>
					</button>
				</div>

				<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-4">
					<div>
						<label class="label">Name</label>
						<input type="text" bind:value={formData.name} required class="input" />
					</div>

					<div>
						<label class="label">Email</label>
						<input type="email" bind:value={formData.email} required class="input" />
					</div>

					<div class="flex items-center">
						<input
							type="checkbox"
							bind:checked={formData.whitelist}
							class="mr-2"
						/>
						<label class="text-sm text-gray-700">
							Whitelist (skip reminders)
						</label>
					</div>

					<div class="flex gap-3">
						<button type="submit" class="btn btn-primary flex-1">
							{editingUser ? 'Update' : 'Add'} User
						</button>
						<button
							type="button"
							onclick={() => (showAddModal = false)}
							class="btn btn-secondary"
						>
							Cancel
						</button>
					</div>
				</form>
			</div>
		</div>
	</div>
{/if}
