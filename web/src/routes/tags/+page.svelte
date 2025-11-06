<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { Tag } from '$lib/types';

	let tags = $state<Tag[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let newTagName = $state('');
	let creating = $state(false);

	onMount(async () => {
		await loadTags();
	});

	async function loadTags() {
		loading = true;
		error = null;

		const response = await api.getTags();
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			tags = response.data;
		}

		loading = false;
	}

	async function handleCreateTag() {
		if (!newTagName.trim()) return;

		creating = true;
		error = null;

		const response = await api.createTag(newTagName.trim());
		if (response.error) {
			error = response.error;
		} else {
			newTagName = '';
			await loadTags();
		}

		creating = false;
	}
</script>

<div class="space-y-6">
	<div class="flex justify-between items-center">
		<h1 class="text-3xl font-bold text-gray-900">Tags</h1>
	</div>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	<!-- Create Tag -->
	<div class="card">
		<h2 class="text-xl font-bold text-gray-900 mb-4">Create New Tag</h2>
		<form onsubmit={(e) => { e.preventDefault(); handleCreateTag(); }} class="flex gap-3">
			<input
				type="text"
				placeholder="Tag name (e.g., Food, Transport, Entertainment)"
				bind:value={newTagName}
				class="input flex-1"
			/>
			<button
				type="submit"
				disabled={!newTagName.trim() || creating}
				class="btn btn-primary disabled:opacity-50"
			>
				{creating ? 'Creating...' : 'Create Tag'}
			</button>
		</form>
	</div>

	<!-- Tags List -->
	<div class="card">
		<h2 class="text-xl font-bold text-gray-900 mb-4">All Tags</h2>

		{#if loading}
			<div class="text-center py-8">
				<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600"></div>
				<p class="mt-2 text-gray-600">Loading tags...</p>
			</div>
		{:else if tags.length === 0}
			<p class="text-gray-500 text-center py-8">
				No tags yet. Create your first tag to categorize transactions!
			</p>
		{:else}
			<div class="flex flex-wrap gap-3">
				{#each tags as tag}
					<div
						class="px-4 py-2 bg-primary-100 text-primary-700 rounded-full font-medium flex items-center gap-2"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"
							/>
						</svg>
						{tag.name}
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>
