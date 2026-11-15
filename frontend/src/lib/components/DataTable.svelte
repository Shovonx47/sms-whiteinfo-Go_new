<script module lang="ts">
	export interface Column<R = any> {
		key: string;
		label: string;
		class?: string;
		render?: (row: R) => string;
		badge?: boolean;
		badgeClass?: (row: R) => string;
		money?: boolean;
	}
</script>

<script lang="ts" generics="T">
	interface Props<R> {
		columns: Column<R>[];
		rows: R[];
		loading?: boolean;
		emptyMessage?: string;
		total?: number;
		perPage?: number;
		currentPage?: number;
		onPageChange?: (page: number) => void;
		perPageOptions?: number[];
		onPerPageChange?: (perPage: number) => void;
		onSort?: (key: string) => void;
		sortKey?: string;
		sortDir?: 'asc' | 'desc';
		searchValue?: string;
		onSearch?: (value: string) => void;
		searchPlaceholder?: string;
		onRowClick?: (row: R) => void;
		groupBy?: (row: R) => string;
		collapsedGroups?: boolean;
	}

	let {
		columns,
		rows,
		loading = false,
		emptyMessage = 'No records found.',
		total = 0,
		perPage = 15,
		currentPage = 1,
		onPageChange,
		perPageOptions,
		onPerPageChange,
		onSort,
		sortKey = '',
		sortDir = 'asc',
		searchValue = $bindable(''),
		onSearch,
		searchPlaceholder = 'Search…',
		onRowClick,
		groupBy,
		collapsedGroups = false
	}: Props<T> = $props();

	let totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));
	let expandedGroups = $state<Record<string, boolean>>({});

	let groupedRows = $derived.by(() => {
		if (!groupBy) return null;
		const groups = new Map<string, T[]>();
		for (const row of rows) {
			const key = groupBy(row);
			const bucket = groups.get(key);
			if (bucket) bucket.push(row);
			else groups.set(key, [row]);
		}
		return Array.from(groups.entries());
	});

	function isGroupOpen(group: string): boolean {
		return expandedGroups[group] ?? !collapsedGroups;
	}

	function toggleGroup(group: string) {
		expandedGroups = { ...expandedGroups, [group]: !isGroupOpen(group) };
	}

	function getCellValue(row: T, col: Column<T>): string {
		if (col.render) return col.render(row);
		const val = (row as Record<string, any>)[col.key];
		if (val === null || val === undefined) return '—';
		if (col.money) {
			return new Intl.NumberFormat('en-US', { minimumFractionDigits: 2 }).format(Number(val));
		}
		return String(val);
	}

	let searchTimeout: ReturnType<typeof setTimeout>;
	function handleSearch(e: Event) {
		const val = (e.target as HTMLInputElement).value;
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => onSearch?.(val), 300);
	}
</script>

<div class="flex flex-col gap-3">
	<!-- Search bar -->
	{#if onSearch !== undefined}
		<div class="relative">
			<svg
				class="absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400"
				fill="none"
				viewBox="0 0 24 24"
				stroke-width="1.5"
				stroke="currentColor"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
				/>
			</svg>
			<input
				type="text"
				value={searchValue}
				oninput={handleSearch}
				placeholder={searchPlaceholder}
				class="w-full rounded-xl border border-gray-300 bg-white py-2 pr-4 pl-9 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 transition focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
			/>
		</div>
	{/if}

	<!-- Table -->
	<div class="overflow-x-auto rounded-xl border border-gray-200/90 bg-white shadow-sm dark:border-gray-800 dark:bg-gray-900">
		<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-800">
			<thead>
				<tr class="bg-gray-50/75 dark:bg-gray-900">
					{#each columns as col (col.key)}
						<th
							class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-600 dark:text-gray-400 {col.class ?? ''} {onSort ? 'cursor-pointer select-none hover:text-gray-900 dark:hover:text-gray-300' : ''}"
							onclick={() => onSort?.(col.key)}
						>
							<div class="flex items-center gap-1.5">
								{col.label}
								{#if onSort && sortKey === col.key}
									<svg
										class="h-3.5 w-3.5 text-teal-600 dark:text-teal-400"
										fill="none"
										viewBox="0 0 24 24"
										stroke-width="2"
										stroke="currentColor"
									>
										{#if sortDir === 'asc'}
											<path stroke-linecap="round" stroke-linejoin="round" d="M4.5 10.5 12 3m0 0 7.5 7.5M12 3v18" />
										{:else}
											<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 13.5 12 21m0 0-7.5-7.5M12 21V3" />
										{/if}
									</svg>
								{/if}
							</div>
						</th>
					{/each}
				</tr>
			</thead>
			<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
				{#snippet dataRow(row: T, index: number)}
					<tr
						class="transition hover:bg-gray-50/75 dark:hover:bg-gray-800/50 {onRowClick ? 'cursor-pointer' : ''}"
						onclick={() => onRowClick?.(row)}
						onkeydown={(e) => { if (onRowClick && e.key === 'Enter') onRowClick(row); }}
						tabindex={onRowClick ? 0 : undefined}
					>
						{#each columns as col (col.key)}
							<td class="px-4 py-3 text-sm text-gray-900 dark:text-gray-300 {col.class ?? ''}">
								{#if col.badge}
									<span
										class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium {col.badgeClass?.(row) ?? 'badge-processing'}"
									>
										{getCellValue(row, col)}
									</span>
								{:else if col.money}
									<span class="font-mono">BDT {getCellValue(row, col)}</span>
								{:else}
									{getCellValue(row, col)}
								{/if}
							</td>
						{/each}
					</tr>
				{/snippet}
				{#if loading}
					{#each { length: 5 } as _, i (i)}
						<tr>
							{#each columns as col (col.key)}
								<td class="px-4 py-3">
									<div class="h-4 animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
								</td>
							{/each}
						</tr>
					{/each}
				{:else if rows.length === 0}
					<tr>
						<td colspan={columns.length} class="px-4 py-12 text-center text-sm text-gray-500">
							{emptyMessage}
						</td>
					</tr>
				{:else if groupedRows}
					{#each groupedRows as [group, groupRows] (group)}
						<tr class="bg-gray-50/80 dark:bg-gray-800/50">
							<td colspan={columns.length} class="px-4 py-2">
								<button
									type="button"
									onclick={() => toggleGroup(group)}
									class="flex w-full items-center gap-2 text-left text-xs font-semibold text-gray-700 dark:text-gray-300"
								>
									<svg class="h-3.5 w-3.5 transition-transform {isGroupOpen(group) ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
										<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
									</svg>
									<span class="capitalize">{group.replaceAll('_', ' ')}</span>
									<span class="rounded-full bg-gray-200 px-2 py-0.5 text-[10px] text-gray-600 dark:bg-gray-700 dark:text-gray-300">{groupRows.length}</span>
								</button>
							</td>
						</tr>
						{#if isGroupOpen(group)}
							{#each groupRows as row, i (i)}
								{@render dataRow(row, i)}
							{/each}
						{/if}
					{/each}
				{:else}
					{#each rows as row, i (i)}
						{@render dataRow(row, i)}
					{/each}
				{/if}
			</tbody>
		</table>
	</div>

	<!-- Pagination -->
	{#if total > 0}
		<div class="flex items-center justify-between text-sm">
			<p class="text-xs text-gray-500">
				Showing {(currentPage - 1) * perPage + 1}–{Math.min(currentPage * perPage, total)} of {total}
			</p>
			<div class="flex items-center gap-3">
				{#if perPageOptions && onPerPageChange}
					<div class="flex items-center gap-1.5">
						<span class="text-xs text-gray-500">Per page</span>
						<select
							value={perPage}
							onchange={(e) => onPerPageChange?.(Number(e.currentTarget.value))}
							class="rounded-lg border border-gray-300 bg-white py-1 px-2 text-xs text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
						>
							{#each perPageOptions as option (option)}
								<option value={option}>{option}</option>
							{/each}
						</select>
					</div>
				{/if}
				<div class="flex items-center gap-1">
				<button
					type="button"
					onclick={() => onPageChange?.(currentPage - 1)}
					disabled={currentPage === 1}
					class="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-sm transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				>
					Prev
				</button>
				{#each { length: Math.min(totalPages, 7) } as _, i (i)}
					{@const p = i + 1}
					<button
						type="button"
						onclick={() => onPageChange?.(p)}
						class="rounded-lg border px-3 py-1.5 text-xs font-semibold transition {p === currentPage
							? 'border-teal-600 bg-teal-50 text-teal-700 dark:border-primary-500 dark:bg-primary-500/20 dark:text-primary-400'
							: 'border-gray-300 bg-white text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300'}"
					>
						{p}
					</button>
				{/each}
				<button
					type="button"
					onclick={() => onPageChange?.(currentPage + 1)}
					disabled={currentPage === totalPages}
					class="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-semibold text-gray-700 shadow-sm transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				>
					Next
				</button>
				</div>
			</div>
		</div>
	{/if}
</div>
