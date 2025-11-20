<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { auth } from "$lib/stores/auth";
	import { api } from "$lib/api/client";

	let error = "";
	let isProcessing = true;

	onMount(async () => {
		const code = $page.url.searchParams.get("code");
		const state = $page.url.searchParams.get("state");
		const errorParam = $page.url.searchParams.get("error");

		if (errorParam) {
			error = errorParam;
			isProcessing = false;
			return;
		}

		if (!code || !state) {
			error = "Missing authorization code or state";
			isProcessing = false;
			return;
		}

		try {
			// Exchange code for token with backend
			const result = await api.handleAuthCallback(code, state);

			if (result.error || !result.data) {
				throw new Error(result.error || "Authentication failed");
			}

			// Store auth data
			auth.login(result.data.token, result.data.expires_at, result.data.user);

			// Redirect to home
			goto("/");
		} catch (e) {
			error = e instanceof Error ? e.message : "Authentication failed";
			isProcessing = false;
		}
	});
</script>

<svelte:head>
	<title>Auth Callback - IMAP Bot</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50">
	<div class="max-w-md w-full space-y-8">
		<div class="text-center">
			{#if isProcessing}
				<div class="inline-block">
					<svg
						class="animate-spin h-12 w-12 text-blue-600"
						xmlns="http://www.w3.org/2000/svg"
						fill="none"
						viewBox="0 0 24 24"
					>
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
						></circle>
						<path
							class="opacity-75"
							fill="currentColor"
							d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
						></path>
					</svg>
				</div>
				<h2 class="mt-6 text-center text-2xl font-bold text-gray-900">Authenticating...</h2>
				<p class="mt-2 text-center text-sm text-gray-600">Please wait while we sign you in</p>
			{:else if error}
				<div class="rounded-md bg-red-50 p-4">
					<div class="flex">
						<div class="flex-shrink-0">
							<svg
								class="h-5 w-5 text-red-400"
								xmlns="http://www.w3.org/2000/svg"
								viewBox="0 0 20 20"
								fill="currentColor"
							>
								<path
									fill-rule="evenodd"
									d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
									clip-rule="evenodd"
								/>
							</svg>
						</div>
						<div class="ml-3">
							<h3 class="text-sm font-medium text-red-800">Authentication Error</h3>
							<div class="mt-2 text-sm text-red-700">
								<p>{error}</p>
							</div>
						</div>
					</div>
				</div>
				<div class="mt-6">
					<a
						href="/login"
						class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
					>
						Back to Login
					</a>
				</div>
			{/if}
		</div>
	</div>
</div>
