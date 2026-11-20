<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';
	import { supermanApi } from '$lib/superman-api.js';

	let refresh = $state(0);

	const columns = [
		{ key: 'created_at', label: 'Date', format: 'datetime' as const },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'gateway_message_id', label: 'Gateway ID', format: 'mono' as const },
		{ key: 'gateway', label: 'Gateway' },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const }
	];

	const actions = [
		{
			label: 'Re-Poll Status',
			tone: 'warning' as const,
			run: async (row: Record<string, any>) => {
				if (!confirm(`Re-poll the delivery status of message #${row.id}?`)) return;
				await supermanApi.post(`/api/v1/superman/dlr/${row.id}/recheck`);
				refresh += 1;
			}
		}
	];
</script>

<svelte:head>
	<title>Failed DLR Re-Check Queue — SMS Whiteinfo Administrator</title>
</svelte:head>

{#key refresh}
	<LegacyTable
		title="Failed DLR Re-Check Queue"
		endpoint="/api/v1/superman/dlr/retry"
		{columns}
		{actions}
		emptyText="There are no failed delivery reports requiring retry."
	/>
{/key}
