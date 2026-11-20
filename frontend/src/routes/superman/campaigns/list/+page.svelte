<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'name', label: 'Campaign Name' },
		{ key: 'customer', label: 'Customer' },
		{ key: 'sender_id', label: 'Sender Header', format: 'badge' as const },
		{ key: 'status', label: 'Status', format: 'badge' as const, align: 'center' as const },
		{ key: 'scheduled_at', label: 'Scheduled / Launch', format: 'datetime' as const },
		{ key: 'created_at', label: 'Created', format: 'datetime' as const }
	];

	const statusOptions = [
		{ value: 'draft', label: 'Draft' },
		{ value: 'scheduled', label: 'Scheduled' },
		{ value: 'paused', label: 'Paused' },
		{ value: 'processing', label: 'Processing' },
		{ value: 'completed', label: 'Completed' },
		{ value: 'failed', label: 'Failed' },
		{ value: 'cancelled', label: 'Cancelled' }
	];
</script>

<svelte:head>
	<title>SMS Marketing &amp; Broadcast Campaigns — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="SMS Marketing & Broadcast Campaigns"
	endpoint="/api/v1/superman/campaigns"
	{columns}
	{statusOptions}
	emptyText="No campaigns found."
/>
