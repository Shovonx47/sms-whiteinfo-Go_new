<script lang="ts">
	import { goto } from '$app/navigation';
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'created_at', label: 'Date', format: 'datetime' as const },
		{ key: 'customer', label: 'Customer' },
		{ key: 'sender_id', label: 'Sender' },
		{ key: 'total_recipients', label: 'Total Recipients', format: 'number' as const, align: 'center' as const },
		{ key: 'valid_count', label: 'Valid', format: 'number' as const, align: 'center' as const },
		{ key: 'invalid_count', label: 'Invalid', format: 'number' as const, align: 'center' as const },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const }
	];

	const actions = [
		{
			label: 'Inspect Batch',
			run: (row: Record<string, any>) => {
				void goto(`/superman/batches?focus=${row.id}`);
			}
		}
	];
</script>

<svelte:head>
	<title>Bulk SMS Campaigns &amp; Batches — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="Bulk SMS Campaigns & Batches"
	endpoint="/api/v1/superman/batches"
	{columns}
	{actions}
	emptyText="No bulk SMS batches found."
/>
