<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';
	import { supermanApi } from '$lib/superman-api.js';

	let refresh = $state(0);

	const columns = [
		{ key: 'customer', label: 'Customer' },
		{ key: 'sender_id', label: 'Sender ID', format: 'badge' as const },
		{ key: 'source', label: 'Source', format: 'badge' as const },
		{ key: 'total_recipients', label: 'Recipients', format: 'number' as const, align: 'right' as const },
		{ key: 'body', label: 'Message Content', format: 'truncate' as const },
		{ key: 'scheduled_at', label: 'Scheduled For', format: 'datetime' as const },
		{ key: 'status', label: 'Status', format: 'badge' as const },
		{ key: 'created_at', label: 'Created', format: 'datetime' as const, hidden: true }
	];

	const statusOptions = [
		{ value: 'scheduled', label: 'Scheduled' },
		{ value: 'paused', label: 'Paused' }
	];

	const actions = [
		{
			label: 'Pause',
			tone: 'warning' as const,
			run: async (row: Record<string, any>) => {
				if (row.status !== 'scheduled') return;
				if (!confirm('Are you sure you want to pause this scheduled message? It will be held and will not send while paused.')) return;
				await supermanApi.post(`/api/v1/superman/batches/${row.id}/pause`);
				refresh += 1;
			}
		},
		{
			label: 'Resume',
			tone: 'success' as const,
			run: async (row: Record<string, any>) => {
				if (row.status !== 'paused') return;
				if (!confirm('Do you want to re-enable this scheduled message? It will automatically dispatch at its scheduled timeline.')) return;
				await supermanApi.post(`/api/v1/superman/batches/${row.id}/resume`);
				refresh += 1;
			}
		},
		{
			label: 'Delete',
			tone: 'danger' as const,
			run: async (row: Record<string, any>) => {
				if (!confirm('Delete this scheduled message? This action is permanent and cannot be undone.')) return;
				await supermanApi.delete(`/api/v1/superman/batches/${row.id}`);
				refresh += 1;
			}
		}
	];
</script>

<svelte:head>
	<title>Scheduled SMS Management — SMS Whiteinfo Administrator</title>
</svelte:head>

{#key refresh}
	<LegacyTable
		title="Scheduled SMS Management"
		endpoint="/api/v1/superman/batches?statuses=scheduled,paused"
		{columns}
		{statusOptions}
		{actions}
		emptyText="No scheduled or paused messages."
	/>
{/key}
