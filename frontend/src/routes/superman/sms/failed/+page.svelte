<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';
	import { supermanApi } from '$lib/superman-api.js';

	let refresh = $state(0);

	const columns = [
		{ key: 'created_at', label: 'Attempted At', format: 'datetime' as const },
		{ key: 'customer', label: 'Customer' },
		{ key: 'recipient', label: 'Recipient', format: 'mono' as const },
		{ key: 'sender_id', label: 'Sender' },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const },
		{ key: 'gateway', label: 'Gateway' },
		{ key: 'cost', label: 'Refunded Cost', format: 'currency' as const, align: 'right' as const }
	];

	const actions = [
		{
			label: 'Requeue',
			tone: 'warning' as const,
			run: async (row: Record<string, any>) => {
				if (!confirm(`Requeue message #${row.id} to the pending queue?`)) return;
				await supermanApi.post(`/api/v1/superman/messages/${row.id}/requeue`);
				refresh += 1;
			}
		}
	];
</script>

<svelte:head>
	<title>Failed SMS &amp; Transmission Errors — SMS Whiteinfo Administrator</title>
</svelte:head>

{#key refresh}
	<LegacyTable
		title="Failed SMS & Transmission Errors"
		endpoint="/api/v1/superman/messages?statuses=failed,dlr_failed"
		{columns}
		{actions}
		emptyText="There are currently no failed messages in the system."
	/>
{/key}
