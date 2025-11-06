<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { Transaction, Tag } from '$lib/types';

	interface Props {
		transaction: Transaction;
		onClose: () => void;
	}

	let { transaction, onClose }: Props = $props();

	let allTags = $state<Tag[]>([]);
	let newTagName = $state('');
	let loading = $state(false);
	let error = $state<string | null>(null);

	onMount(async () => {
		await loadTags();
	});

	async function loadTags() {
		const response = await api.getTags();
		if (response.data) {
			allTags = response.data;
		}
	}

	async function handleAddTag(tagId: number) {
		loading = true;
		error = null;

		const response = await api.addTagToTransaction(transaction.id, tagId);
		if (response.error) {
			error = response.error;
		} else {
			onClose();
		}

		loading = false;
	}

	async function handleRemoveTag(tagId: number) {
		loading = true;
		error = null;

		const response = await api.removeTagFromTransaction(transaction.id, tagId);
		if (response.error) {
			error = response.error;
		} else {
			onClose();
		}

		loading = false;
	}

	async function handleCreateTag() {
		if (!newTagName.trim()) return;

		loading = true;
		error = null;

		const response = await api.createTag(newTagName.trim());
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			await loadTags();
			newTagName = '';
			if (response.data.id) {
				await handleAddTag(response.data.id);
			}
		}

		loading = false;
	}

	function isTagApplied(tagId: number): boolean {
		return transaction.tags?.some((t) => t.id === tagId) || false;
	}
</script>

<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
	<div class="bg-white rounded-lg shadow-xl max-w-md w-full">
		<div class="p-6">
			<div class="flex justify-between items-center mb-6">
				<h2 class="text-2xl font-bold text-gray-900">Manage Tags</h2>
				<button onclick={onClose} class="text-gray-500 hover:text-gray-700">
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

			{#if error}
				<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-4">
					{error}
				</div>
			{/if}

			<!-- Create New Tag -->
			<div class="mb-6">
				<label class="label">Create New Tag</label>
				<div class="flex gap-2">
					<input
						type="text"
						placeholder="Tag name"
						bind:value={newTagName}
						class="input flex-1"
						onkeypress={(e) => e.key === 'Enter' && handleCreateTag()}
					/>
					<button
						onclick={handleCreateTag}
						disabled={!newTagName.trim() || loading}
						class="btn btn-primary disabled:opacity-50"
					>
						Add
					</button>
				</div>
			</div>

			<!-- Existing Tags -->
			<div>
				<label class="label">Available Tags</label>
				<div class="space-y-2 max-h-60 overflow-y-auto">
					{#if allTags.length === 0}
						<p class="text-gray-500 text-center py-4">No tags yet</p>
					{:else}
						{#each allTags as tag}
							<div class="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
								<span class="font-medium text-gray-900">{tag.name}</span>
								{#if isTagApplied(tag.id)}
									<button
										onclick={() => handleRemoveTag(tag.id)}
										disabled={loading}
										class="px-3 py-1 bg-red-100 text-red-700 rounded-full text-sm hover:bg-red-200 disabled:opacity-50"
									>
										Remove
									</button>
								{:else}
									<button
										onclick={() => handleAddTag(tag.id)}
										disabled={loading}
										class="px-3 py-1 bg-primary-100 text-primary-700 rounded-full text-sm hover:bg-primary-200 disabled:opacity-50"
									>
										Add
									</button>
								{/if}
							</div>
						{/each}
					{/if}
				</div>
			</div>

			<!-- Actions -->
			<div class="mt-6">
				<button onclick={onClose} class="btn btn-secondary w-full">Close</button>
			</div>
		</div>
	</div>
</div>
