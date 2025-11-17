<script lang="ts">
	let apiUrl = $state("http://localhost:8080/api");
	let saved = $state(false);

	function saveSettings() {
		// In a real app, this would save to localStorage or backend
		localStorage.setItem("apiUrl", apiUrl);
		saved = true;
		setTimeout(() => {
			saved = false;
		}, 3000);
	}

	function loadSettings() {
		const savedUrl = localStorage.getItem("apiUrl");
		if (savedUrl) {
			apiUrl = savedUrl;
		}
	}

	$effect(() => {
		loadSettings();
	});
</script>

<div class="space-y-6">
	<h1 class="text-3xl font-bold text-gray-900">Settings</h1>

	{#if saved}
		<div class="bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded-lg">
			Settings saved successfully!
		</div>
	{/if}

	<!-- API Settings -->
	<div class="card">
		<h2 class="text-xl font-bold text-gray-900 mb-4">API Configuration</h2>
		<div class="space-y-4">
			<div>
				<label class="label" for="api-url">API Base URL</label>
				<input id="api-url" type="text" bind:value={apiUrl} class="input" />
				<p class="text-sm text-gray-500 mt-1">
					The base URL for the backend API (default: http://localhost:8080/api)
				</p>
			</div>
			<button onclick={saveSettings} class="btn btn-primary">Save Settings</button>
		</div>
	</div>

	<!-- About -->
	<div class="card">
		<h2 class="text-xl font-bold text-gray-900 mb-4">About</h2>
		<div class="space-y-2 text-gray-600">
			<p><strong>Bill Splitter</strong> - Expense Tracking and Bill Splitting Application</p>
			<p class="text-sm">
				This application helps you track expenses from bank emails, split bills among friends, and
				send payment reminders.
			</p>
		</div>
	</div>

	<!-- Features -->
	<div class="card">
		<h2 class="text-xl font-bold text-gray-900 mb-4">Features</h2>
		<ul class="space-y-2 text-gray-600">
			<li class="flex items-start">
				<svg
					class="w-5 h-5 text-green-500 mr-2 mt-0.5"
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
				Automatic transaction tracking from bank emails
			</li>
			<li class="flex items-start">
				<svg
					class="w-5 h-5 text-green-500 mr-2 mt-0.5"
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
				Flexible bill splitting (equal or custom amounts)
			</li>
			<li class="flex items-start">
				<svg
					class="w-5 h-5 text-green-500 mr-2 mt-0.5"
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
				Payment reminders with QR codes
			</li>
			<li class="flex items-start">
				<svg
					class="w-5 h-5 text-green-500 mr-2 mt-0.5"
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
				Transaction tagging and categorization
			</li>
			<li class="flex items-start">
				<svg
					class="w-5 h-5 text-green-500 mr-2 mt-0.5"
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
				Statistics and spending analysis
			</li>
			<li class="flex items-start">
				<svg
					class="w-5 h-5 text-green-500 mr-2 mt-0.5"
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
				User management with whitelist support
			</li>
		</ul>
	</div>
</div>
