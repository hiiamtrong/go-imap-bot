<script lang="ts">
	import { page } from '$app/stores';

	const navItems = [
		{ path: '/', label: 'Dashboard' },
		{ path: '/transactions', label: 'Transactions' },
		{ path: '/users', label: 'Users' },
		{ path: '/tags', label: 'Tags' },
		{ path: '/statistics', label: 'Statistics' },
		{ path: '/reminders', label: 'Reminders' },
		{ path: '/settings', label: 'Settings' }
	];
</script>

<nav class="bg-white shadow-lg">
	<div class="container mx-auto px-4">
		<div class="flex items-center justify-between h-16">
			<div class="flex items-center space-x-2">
				<svg
					class="w-8 h-8 text-primary-600"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="2"
						d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
					/>
				</svg>
				<span class="text-xl font-bold text-gray-800">Bill Splitter</span>
			</div>

			<div class="hidden md:flex space-x-1">
				{#each navItems as item}
					{@const isActive = item.path === '/'
						? $page.url.pathname === '/'
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
			</div>

			<!-- Mobile menu button -->
			<div class="md:hidden">
				<button class="text-gray-700 hover:text-gray-900" aria-label="Toggle menu">
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M4 6h16M4 12h16M4 18h16"
						/>
					</svg>
				</button>
			</div>
		</div>
	</div>
</nav>
