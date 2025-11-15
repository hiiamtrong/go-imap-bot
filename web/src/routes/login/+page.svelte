<script lang="ts">
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { auth } from "$lib/stores/auth";
  import { api } from "$lib/api/client";

  let isLoading = false;
  let error = "";

  onMount(() => {
    // If already authenticated, redirect to home
    const authState = $auth;
    if (authState.isAuthenticated) {
      goto("/");
    }
  });

  async function handleLogin() {
    isLoading = true;
    error = "";
    try {
      await api.login();
    } catch (e) {
      error = e instanceof Error ? e.message : "Login failed";
      isLoading = false;
    }
  }
</script>

<svelte:head>
  <title>Login - IMAP Bot</title>
</svelte:head>

<div
  class="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8"
>
  <div class="max-w-md w-full space-y-8">
    <div>
      <h2 class="mt-6 text-center text-3xl font-extrabold text-gray-900">
        Sign in to IMAP Bot
      </h2>
      <p class="mt-2 text-center text-sm text-gray-600">
        Use your Google account to continue
      </p>
    </div>
    <div class="mt-8 space-y-6">
      {#if error}
        <div class="rounded-md bg-red-50 p-4">
          <div class="text-sm text-red-700">
            {error}
          </div>
        </div>
      {/if}
      <div>
        <button
          type="button"
          on:click={handleLogin}
          disabled={isLoading}
          class="group relative w-full flex justify-center py-2 px-4 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <span class="absolute left-0 inset-y-0 flex items-center pl-3">
            <svg
              class="h-5 w-5 text-blue-500 group-hover:text-blue-400"
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                d="M12.48 10.92v3.28h7.84c-.24 1.84-.853 3.187-1.787 4.133-1.147 1.147-2.933 2.4-6.053 2.4-4.827 0-8.6-3.893-8.6-8.72s3.773-8.72 8.6-8.72c2.6 0 4.507 1.027 5.907 2.347l2.307-2.307C18.747 1.44 16.133 0 12.48 0 5.867 0 .307 5.387.307 12s5.56 12 12.173 12c3.573 0 6.267-1.173 8.373-3.36 2.16-2.16 2.84-5.213 2.84-7.667 0-.76-.053-1.467-.173-2.053H12.48z"
              />
            </svg>
          </span>
          {isLoading ? "Loading..." : "Sign in with Google"}
        </button>
      </div>
      <div class="text-center">
        <p class="text-xs text-gray-500">
          By signing in, you agree to our terms of service
        </p>
      </div>
    </div>
  </div>
</div>
