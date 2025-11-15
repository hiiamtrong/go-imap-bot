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

  onMount(() => {
    // Check if current route requires authentication
    const currentPath = $page.url.pathname;
    const isPublicRoute = publicRoutes.some((route) =>
      currentPath.startsWith(route)
    );

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
  <title>Bill Splitter - Expense Tracking</title>
</svelte:head>

<div class="min-h-screen bg-gray-50">
  <Navigation />
  <main class="container mx-auto px-4 py-8">
    {@render children()}
  </main>
</div>
