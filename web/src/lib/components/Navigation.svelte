<script lang="ts">
	import { page } from "$app/stores";
	import { auth, currentUser } from "$lib/stores/auth";

	const navItems = [
		{ path: "/", label: "Dashboard" },
		{ path: "/transactions", label: "Transactions" },
		{ path: "/users", label: "Users" },
		{ path: "/tags", label: "Tags" },
		{ path: "/statistics", label: "Statistics" },
		{ path: "/reminders", label: "Reminders" },
		{ path: "/settings", label: "Settings" },
	];

	let isMobileMenuOpen = false;

	function toggleMobileMenu() {
		isMobileMenuOpen = !isMobileMenuOpen;
	}

	function closeMobileMenu() {
		isMobileMenuOpen = false;
	}

	function handleLogout() {
		auth.logout();
		window.location.href = "/login";
	}
</script>

<nav class="bg-white shadow-lg sticky top-0 z-50">
	<div class="container mx-auto px-4">
		<div class="flex items-center justify-between h-16">
			<div class="flex items-center space-x-2">
				<svg class="w-8 h-8 text-primary-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
					/>
				</svg>
				<span class="text-xl font-bold text-gray-800">Bill Splitter</span>
			</div>

			<div class="hidden md:flex items-center space-x-1">
				{#each navItems as item}
					{@const isActive =
						item.path === "/"
							? $page.url.pathname === "/"
							: $page.url.pathname.startsWith(item.path)}
					<a
						href={item.path}
						class="px-3 py-2 rounded-md text-sm font-medium transition-colors"
						class:bg-blue-600={isActive}
						class:text-white={isActive}
						class:text-gray-700={!isActive}
						class:hover:bg-gray-100={!isActive}
					>
						{item.label}
					</a>
				{/each}

				{#if $auth.isAuthenticated && $currentUser}
					<div class="ml-4 flex items-center space-x-3 border-l pl-4">
						<span class="text-sm text-gray-700">{$currentUser.name}</span>
						<button
							on:click={handleLogout}
							class="px-3 py-2 rounded-md text-sm font-medium text-gray-700 hover:bg-red-50 hover:text-red-600 transition-colors"
						>
							Logout
						</button>
					</div>
				{/if}
			</div>

			<!-- Mobile menu button -->
			<div class="md:hidden">
				<button
					on:click={toggleMobileMenu}
					class="text-gray-700 hover:text-gray-900"
					aria-label="Toggle menu"
				>
					{#if isMobileMenuOpen}
						<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M6 18L18 6M6 6l12 12"
							/>
						</svg>
					{:else}
						<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="2"
								d="M4 6h16M4 12h16M4 18h16"
							/>
						</svg>
					{/if}
				</button>
			</div>
		</div>

		<!-- Mobile menu -->
		{#if isMobileMenuOpen}
			<div class="md:hidden border-t border-gray-200">
				<div class="px-2 pt-2 pb-3 space-y-1">
					{#each navItems as item}
						{@const isActive =
							item.path === "/"
								? $page.url.pathname === "/"
								: $page.url.pathname.startsWith(item.path)}
						<a
							href={item.path}
							on:click={closeMobileMenu}
							class="block px-3 py-2 rounded-md text-base font-medium transition-colors"
							class:bg-blue-600={isActive}
							class:text-white={isActive}
							class:text-gray-700={!isActive}
							class:hover:bg-gray-100={!isActive}
						>
							{item.label}
						</a>
					{/each}

					{#if $auth.isAuthenticated && $currentUser}
						<div class="pt-4 border-t border-gray-200 mt-4">
							<div class="px-3 py-2">
								<p class="text-sm font-medium text-gray-900">
									{$currentUser.name}
								</p>
								<p class="text-sm text-gray-500">{$currentUser.email}</p>
							</div>
							<button
								on:click={handleLogout}
								class="block w-full text-left px-3 py-2 rounded-md text-base font-medium text-gray-700 hover:bg-red-50 hover:text-red-600 transition-colors"
							>
								Logout
							</button>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</div>
</nav>
