<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import "../app.css";
	import favicon from "$lib/assets/favicon.svg";
	import Navigation from "$lib/components/Navigation.svelte";
	import { auth } from "$lib/stores/auth";

	let { children } = $props();

	// Public routes that don't require authentication
	const publicRoutes = ["/login", "/auth/callback"];

	// Page title mapping
	const pageTitles: Record<string, string> = {
		"/": "Home",
		"/login": "Login",
		"/auth/callback": "Auth Callback",
		"/tags": "Tags",
		"/users": "Users",
		"/transactions": "Transactions",
		"/transactions/new": "New Transaction",
		"/statistics": "Statistics",
		"/settings": "Settings",
		"/reminders": "Reminders"
	};

	// Get page title based on current path
	function getPageTitle(pathname: string): string {
		// Check exact match first
		if (pageTitles[pathname]) {
			return pageTitles[pathname];
		}

		// Check for dynamic routes like /transactions/[id]
		if (pathname.startsWith("/transactions/") && pathname !== "/transactions/new") {
			return "Transaction Details";
		}

		// Default title
		return "Bill Splitter";
	}

	$: pageTitle = getPageTitle($page.url.pathname);

	onMount(() => {
		// Check if current route requires authentication
		const currentPath = $page.url.pathname;
		const isPublicRoute = publicRoutes.some((route) => currentPath.startsWith(route));

		// If not authenticated and not on a public route, redirect to login
		if (!$auth.isAuthenticated && !isPublicRoute) {
			goto("/login");
		}

		// If authenticated and on login page, redirect to home
		if ($auth.isAuthenticated && currentPath === "/login") {
			goto("/");
		}
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{pageTitle} - IMAP Bot</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
	{#if $auth.isAuthenticated}
		<Navigation />
	{/if}
	<main class="container mx-auto px-4 py-8">
		{@render children()}
	</main>
</div>
