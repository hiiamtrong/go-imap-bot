<script lang="ts">
	import { onMount } from "svelte";
	import { api } from "$lib/api/client";
	import { formatCurrency } from "$lib/utils/format";
	import type { Statistics } from "$lib/types";
	import Chart from "chart.js/auto";

	let stats = $state<Statistics | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let monthlyChartCanvas = $state<HTMLCanvasElement>();
	let tagChartCanvas = $state<HTMLCanvasElement>();
	let monthlyChart: Chart | null = null;
	let tagChart: Chart | null = null;

	onMount(() => {
		loadStatistics();
		return () => {
			if (monthlyChart) monthlyChart.destroy();
			if (tagChart) tagChart.destroy();
		};
	});

	async function loadStatistics() {
		loading = true;
		error = null;

		const response = await api.getStatistics();
		if (response.error) {
			error = response.error;
		} else if (response.data) {
			stats = response.data;
			setTimeout(() => {
				createCharts();
			}, 100);
		}

		loading = false;
	}

	function createCharts() {
		if (!stats) return;

		// Monthly Spending Chart
		if (monthlyChartCanvas && stats.spendingByMonth) {
			if (monthlyChart) monthlyChart.destroy();

			monthlyChart = new Chart(monthlyChartCanvas, {
				type: "bar",
				data: {
					labels: stats.spendingByMonth.map((m) => m.month),
					datasets: [
						{
							label: "Spending",
							data: stats.spendingByMonth.map((m) => m.amount),
							backgroundColor: "rgba(14, 165, 233, 0.5)",
							borderColor: "rgba(14, 165, 233, 1)",
							borderWidth: 1,
						},
					],
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					scales: {
						y: {
							beginAtZero: true,
							ticks: {
								callback: function (value) {
									return formatCurrency(Number(value), "VND");
								},
							},
						},
					},
					plugins: {
						tooltip: {
							callbacks: {
								label: function (context) {
									return formatCurrency(context.parsed.y ?? 0, "VND");
								},
							},
						},
					},
				},
			});
		}

		// Tag Spending Chart
		if (tagChartCanvas && stats.spendingByTag) {
			if (tagChart) tagChart.destroy();

			tagChart = new Chart(tagChartCanvas, {
				type: "doughnut",
				data: {
					labels: stats.spendingByTag.map((t) => t.tag),
					datasets: [
						{
							data: stats.spendingByTag.map((t) => t.amount),
							backgroundColor: [
								"rgba(14, 165, 233, 0.8)",
								"rgba(34, 197, 94, 0.8)",
								"rgba(251, 146, 60, 0.8)",
								"rgba(249, 115, 22, 0.8)",
								"rgba(168, 85, 247, 0.8)",
								"rgba(236, 72, 153, 0.8)",
							],
							borderWidth: 2,
						},
					],
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					plugins: {
						tooltip: {
							callbacks: {
								label: function (context) {
									return `${context.label}: ${formatCurrency(context.parsed, "VND")}`;
								},
							},
						},
					},
				},
			});
		}
	}
</script>

<div class="space-y-6">
	<h1 class="text-3xl font-bold text-gray-900">Statistics</h1>

	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg">
			{error}
		</div>
	{/if}

	{#if loading}
		<div class="card text-center py-12">
			<div
				class="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"
			></div>
			<p class="mt-4 text-gray-600">Loading statistics...</p>
		</div>
	{:else if stats}
		<!-- Overview Cards -->
		<div class="grid grid-cols-1 md:grid-cols-4 gap-6">
			<div class="card">
				<p class="text-sm text-gray-600">Total Spent</p>
				<p class="text-2xl font-bold text-red-600">{formatCurrency(stats.totalSpent, "VND")}</p>
			</div>

			<div class="card">
				<p class="text-sm text-gray-600">Total Received</p>
				<p class="text-2xl font-bold text-green-600">{formatCurrency(stats.totalReceived, "VND")}</p>
			</div>

			<div class="card">
				<p class="text-sm text-gray-600">Current Balance</p>
				<p class="text-2xl font-bold text-primary-600">{formatCurrency(stats.balance, "VND")}</p>
			</div>

			<div class="card">
				<p class="text-sm text-gray-600">Total Transactions</p>
				<p class="text-2xl font-bold text-gray-900">{stats.transactionCount}</p>
			</div>
		</div>

		<!-- Split Status -->
		<div class="grid grid-cols-1 md:grid-cols-2 gap-6">
			<div class="card">
				<h3 class="text-lg font-bold text-gray-900 mb-2">Split Status</h3>
				<div class="space-y-2">
					<div class="flex justify-between">
						<span class="text-gray-600">Pending Splits</span>
						<span class="font-medium text-orange-600">{stats.pendingSplits}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-gray-600">Completed Splits</span>
						<span class="font-medium text-green-600">{stats.completedSplits}</span>
					</div>
					<div class="pt-2 border-t border-gray-200">
						<div class="flex justify-between">
							<span class="text-gray-900 font-medium">Total Splits</span>
							<span class="font-bold text-gray-900">
								{stats.pendingSplits + stats.completedSplits}
							</span>
						</div>
					</div>
				</div>
			</div>

			<div class="card">
				<h3 class="text-lg font-bold text-gray-900 mb-2">Completion Rate</h3>
				<div class="mt-4">
					{#if stats.pendingSplits + stats.completedSplits > 0}
						{@const completionRate =
							(stats.completedSplits / (stats.pendingSplits + stats.completedSplits)) * 100}
						<div class="mb-2">
							<div class="w-full bg-gray-200 rounded-full h-4">
								<div
									class="bg-primary-600 h-4 rounded-full transition-all"
									style="width: {completionRate}%"
								></div>
							</div>
						</div>
						<p class="text-2xl font-bold text-primary-600">{completionRate.toFixed(1)}%</p>
					{:else}
						<p class="text-gray-500">No splits yet</p>
					{/if}
				</div>
			</div>
		</div>

		<!-- Charts -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			{#if stats.spendingByMonth && stats.spendingByMonth.length > 0}
				<div class="card">
					<h3 class="text-lg font-bold text-gray-900 mb-4">Monthly Spending</h3>
					<div class="h-80">
						<canvas bind:this={monthlyChartCanvas}></canvas>
					</div>
				</div>
			{/if}

			{#if stats.spendingByTag && stats.spendingByTag.length > 0}
				<div class="card">
					<h3 class="text-lg font-bold text-gray-900 mb-4">Spending by Category</h3>
					<div class="h-80">
						<canvas bind:this={tagChartCanvas}></canvas>
					</div>
				</div>
			{/if}
		</div>

		<!-- Detailed Tables -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			{#if stats.spendingByMonth && stats.spendingByMonth.length > 0}
				<div class="card">
					<h3 class="text-lg font-bold text-gray-900 mb-4">Monthly Breakdown</h3>
					<div class="space-y-2">
						{#each stats.spendingByMonth as month}
							<div class="flex justify-between items-center p-2 bg-gray-50 rounded">
								<span class="text-gray-700">{month.month}</span>
								<div class="text-right">
									<p class="font-medium">{formatCurrency(month.amount, "VND")}</p>
									<p class="text-xs text-gray-500">{month.count} transactions</p>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{/if}

			{#if stats.spendingByTag && stats.spendingByTag.length > 0}
				<div class="card">
					<h3 class="text-lg font-bold text-gray-900 mb-4">Category Breakdown</h3>
					<div class="space-y-2">
						{#each stats.spendingByTag as tag}
							<div class="flex justify-between items-center p-2 bg-gray-50 rounded">
								<span class="text-gray-700">{tag.tag}</span>
								<div class="text-right">
									<p class="font-medium">{formatCurrency(tag.amount, "VND")}</p>
									<p class="text-xs text-gray-500">{tag.count} transactions</p>
								</div>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>
