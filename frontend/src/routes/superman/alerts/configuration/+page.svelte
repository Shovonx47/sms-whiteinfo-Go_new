<script lang="ts">
	let lowBalanceThreshold = $state(50);
	let failureRatePercent = $state(10);
	let consecutiveFailureCount = $state(5);
	let saved = $state(false);

	function save() {
		// The old Filament page only confirmed the save; thresholds were not persisted.
		saved = true;
		setTimeout(() => (saved = false), 3000);
	}
</script>

<svelte:head>
	<title>Alert Rules &amp; Thresholds Configuration — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-4">
	<h1 class="text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Alert Rules & Thresholds Configuration</h1>

	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="border-b border-gray-100 px-6 py-4 dark:border-gray-800">
			<h2 class="text-sm font-bold text-gray-900 dark:text-white">Automated Alert Thresholds &amp; Health Watchdogs</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
				Customize system trigger points for generating operations notifications and warnings.
			</p>
		</div>

		<form
			class="space-y-6 px-6 py-5"
			onsubmit={(event) => {
				event.preventDefault();
				save();
			}}
		>
			<div class="grid grid-cols-1 gap-6 md:grid-cols-3">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="low-balance">Customer Low Balance Alert (BDT)</label>
					<input
						id="low-balance"
						type="number"
						step="0.5"
						bind:value={lowBalanceThreshold}
						class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
					/>
					<p class="mt-1 text-xs text-gray-500">Triggers low balance warning when wallet falls below this value.</p>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="failure-rate">Gateway Failure Rate (%)</label>
					<input
						id="failure-rate"
						type="number"
						min="1"
						max="100"
						bind:value={failureRatePercent}
						class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
					/>
					<p class="mt-1 text-xs text-gray-500">Generates alert if failures exceed this percentage of total traffic.</p>
				</div>

				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="consecutive-failures">Consecutive Route Failures</label>
					<input
						id="consecutive-failures"
						type="number"
						min="1"
						max="50"
						bind:value={consecutiveFailureCount}
						class="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-900 dark:text-white"
					/>
					<p class="mt-1 text-xs text-gray-500">Number of failed sends before auto-rerouting to backup gateway.</p>
				</div>
			</div>

			{#if saved}
				<p class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300">
					Alert thresholds saved. System alert triggers updated successfully.
				</p>
			{/if}

			<div class="border-t border-gray-200 pt-4 dark:border-gray-700">
				<button type="submit" class="rounded-lg bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700">
					Save Alert Triggers
				</button>
			</div>
		</form>
	</div>
</div>
