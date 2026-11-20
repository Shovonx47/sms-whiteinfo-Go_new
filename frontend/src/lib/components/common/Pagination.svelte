<script lang="ts">
	interface Props {
		total: number;
		page: number;
		perPage: number;
		perPageOptions?: number[];
		onPageChange: (page: number) => void;
		onPerPageChange?: (perPage: number) => void;
		loading?: boolean;
		class?: string;
	}

	let {
		total,
		page,
		perPage,
		perPageOptions = [10, 25, 50, 100],
		onPageChange,
		onPerPageChange,
		loading = false,
		class: customClass = ''
	}: Props = $props();

	let totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));
	let from = $derived(total === 0 ? 0 : (page - 1) * perPage + 1);
	let to = $derived(Math.min(page * perPage, total));

	// 1 2 3 4 … 492 493 — always keeps the first/last two pages and a window around the current page.
	let pages = $derived.by<(number | 'gap')[]>(() => {
		const last = totalPages;
		if (last <= 7) return Array.from({ length: last }, (_, i) => i + 1);

		const keep = new Set<number>([1, 2, last - 1, last]);
		for (let p = page - 1; p <= page + 1; p++) {
			if (p >= 1 && p <= last) keep.add(p);
		}
		if (page <= 3) [3, 4].forEach((p) => keep.add(p));
		if (page >= last - 2) [last - 2, last - 3].forEach((p) => keep.add(p));

		const sorted = [...keep].filter((p) => p >= 1 && p <= last).sort((a, b) => a - b);
		const out: (number | 'gap')[] = [];
		sorted.forEach((p, index) => {
			if (index > 0 && p - sorted[index - 1] > 1) out.push('gap');
			out.push(p);
		});
		return out;
	});

	function go(target: number) {
		if (loading || target < 1 || target > totalPages || target === page) return;
		onPageChange(target);
	}
</script>

<div
	class="flex flex-col gap-3 border-t border-gray-100 px-5 py-4 text-xs text-gray-600 dark:border-gray-800 dark:text-gray-400 sm:flex-row sm:items-center sm:justify-between {customClass}"
	data-testid="pagination"
>
	<p data-testid="pagination-summary">
		Showing <span class="font-semibold text-gray-900 dark:text-white">{from.toLocaleString()}</span>
		to <span class="font-semibold text-gray-900 dark:text-white">{to.toLocaleString()}</span>
		of <span class="font-semibold text-gray-900 dark:text-white">{total.toLocaleString()}</span> results
	</p>

	<div class="flex flex-wrap items-center gap-3">
		{#if onPerPageChange}
			<label class="flex items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 py-1.5 shadow-xs dark:border-gray-700 dark:bg-gray-900">
				<span>Per page</span>
				<select
					value={perPage}
					onchange={(e) => onPerPageChange?.(Number(e.currentTarget.value))}
					class="bg-transparent pr-1 text-xs font-semibold text-gray-900 outline-none dark:text-white"
					aria-label="Rows per page"
				>
					{#each perPageOptions as option (option)}
						<option value={option}>{option}</option>
					{/each}
				</select>
			</label>
		{/if}

		{#if totalPages > 1}
			<nav class="flex items-center gap-1 rounded-lg border border-gray-200 bg-white p-1 shadow-xs dark:border-gray-700 dark:bg-gray-900" aria-label="Pagination">
				{#if page > 1}
					<button
						type="button"
						onclick={() => go(page - 1)}
						aria-label="Previous page"
						class="rounded-md px-2 py-1.5 text-gray-600 transition hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800"
					>
						<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m15.75 19.5-7.5-7.5 7.5-7.5" /></svg>
					</button>
				{/if}
				{#each pages as item, index (index)}
					{#if item === 'gap'}
						<span class="px-2 text-gray-400">…</span>
					{:else}
						<button
							type="button"
							onclick={() => go(item)}
							aria-current={item === page ? 'page' : undefined}
							class="min-w-8 rounded-md px-2.5 py-1.5 text-xs font-semibold transition {item === page
								? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-600/30 dark:bg-emerald-950/40 dark:text-emerald-300'
								: 'text-gray-700 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-800'}"
						>
							{item}
						</button>
					{/if}
				{/each}
				<button
					type="button"
					onclick={() => go(page + 1)}
					disabled={page >= totalPages}
					aria-label="Next page"
					class="rounded-md px-2 py-1.5 text-gray-600 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-300 dark:hover:bg-gray-800"
				>
					<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" /></svg>
				</button>
			</nav>
		{/if}
	</div>
</div>
