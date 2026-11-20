<script lang="ts">
	import LegacyTable from '$lib/components/admin/LegacyTable.svelte';

	const columns = [
		{ key: 'created_at', label: 'Timestamp', format: 'datetime' as const },
		{ key: 'event', label: 'Event', format: 'badge' as const },
		{ key: 'phone', label: 'Target MSISDN', format: 'mono' as const },
		{ key: 'gateway_name', label: 'Gateway' },
		{ key: 'status', label: 'Status', format: 'badge' as const },
		{ key: 'duration_ms', label: 'Latency (ms)', format: 'number' as const, align: 'right' as const },
		{ key: 'error_message', label: 'Error Trace', format: 'truncate' as const }
	];
</script>

<svelte:head>
	<title>SMS Transmission &amp; Audit Logs — SMS Whiteinfo Administrator</title>
</svelte:head>

<LegacyTable
	title="SMS Transmission & Audit Logs"
	endpoint="/api/v1/superman/sms/logs"
	{columns}
	emptyText="No SMS logs recorded yet."
/>
