<script lang="ts">
	const base = 'https://sms.whiteinfobd.com';

	type Endpoint = {
		method: 'GET' | 'POST' | 'DELETE';
		path: string;
		description: string;
		body?: string;
		response?: string;
	};

	const endpoints: { group: string; items: Endpoint[] }[] = [
		{
			group: 'Account',
			items: [
				{
					method: 'GET',
					path: '/api/v1/me',
					description: 'Return the authenticated customer, wallet balance and account settings.',
					response: `{
  "status": "success",
  "code": 200,
  "message": "OK",
  "data": { "id": 1, "name": "...", "email": "...", "balance": "1000.00", "api_access": true }
}`
				},
				{
					method: 'GET',
					path: '/api/v1/balance',
					description: 'Return the current wallet balance.',
					response: `{ "status": "success", "code": 200, "message": "OK", "data": { "balance": "1000.00" } }`
				},
				{
					method: 'GET',
					path: '/api/v1/senders',
					description: 'List the approved Sender IDs assigned to your account.'
				}
			]
		},
		{
			group: 'SMS',
			items: [
				{
					method: 'POST',
					path: '/api/v1/messages',
					description:
						'Send an SMS. Accepts a single recipient (phone, to or message alias) or a comma-separated recipient list. template_id bypasses the body requirement using the saved template content.',
					body: `{
  "api_token": "<token>",
  "sender_id": "SENDER",
  "phone": "8801700000000",
  "message": "Hello from SMS Whiteinfo",
  "template_id": null,
  "schedule_at": "2026-11-20 10:30:00"
}`,
					response: `{
  "status": "success",
  "code": 200,
  "message": "SMS sent successfully.",
  "data": { "batch_id": 123, "recipients": 1, "cost": "0.2500", "balance": "999.7500" }
}`
				},
				{
					method: 'GET',
					path: '/api/v1/messages',
					description:
						'Send with query parameters (same fields as the POST body). Useful for simple integrations that only support GET.',
					body: `?api_token=<token>&sender_id=SENDER&phone=8801700000000&message=Hello`
				},
				{
					method: 'GET',
					path: '/api/v1/batches/{batch}',
					description: 'Read the status and per-recipient delivery report of a batch you own.',
					response: `{
  "status": "success",
  "code": 200,
  "message": "OK",
  "data": {
    "id": 123, "status": "completed", "total_recipients": 1,
    "messages": [{ "to": "8801700000000", "status": "delivered", "cost": "0.2500" }]
  }
}`
				}
			]
		},
		{
			group: 'IP Whitelist',
			items: [
				{
					method: 'GET',
					path: '/api/v1/ip-whitelists',
					description: 'List the IP addresses allowed to call the developer API for this account.'
				},
				{
					method: 'POST',
					path: '/api/v1/ip-whitelists',
					description: 'Request whitelisting of an IP address. Requests require administrator approval.',
					body: `{ "ip_address": "203.0.113.10", "label": "Production server" }`
				},
				{
					method: 'DELETE',
					path: '/api/v1/ip-whitelists/{id}',
					description: 'Remove an IP whitelist entry you own.'
				}
			]
		}
	];

	function methodClass(method: Endpoint['method']) {
		if (method === 'POST') return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800';
		if (method === 'DELETE') return 'bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-800';
		return 'bg-sky-50 text-sky-700 border-sky-200 dark:bg-sky-950/40 dark:text-sky-300 dark:border-sky-800';
	}
</script>

<svelte:head>
	<title>API Documentation – SMS Whiteinfo</title>
	<meta name="description" content="Developer API documentation for SMS Whiteinfo customers." />
</svelte:head>

<div class="mx-auto max-w-5xl space-y-8 p-6">
	<header class="border-b border-gray-200 pb-5 dark:border-gray-800">
		<h1 class="text-2xl font-bold text-gray-950 dark:text-white">API Documentation</h1>
		<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
			Integrate SMS Whiteinfo with your application. All developer API requests require a Bearer access token and, when enabled on your account, an approved source IP address.
		</p>
		<div class="mt-4 flex flex-wrap gap-3 text-xs">
			<a href="/developer/access-tokens" class="rounded-lg bg-teal-600 px-3 py-2 font-semibold text-white hover:bg-teal-500">Manage Access Tokens</a>
			<a href="/developer/ip-whitelists" class="rounded-lg border border-gray-300 px-3 py-2 font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800">IP Whitelist</a>
		</div>
	</header>

	<section class="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-800 dark:bg-gray-900">
		<h2 class="text-sm font-bold uppercase tracking-wider text-gray-900 dark:text-white">Authentication</h2>
		<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">Send your token in the <code class="font-mono text-xs">Authorization</code> header. Tokens are created on the Access Tokens page and are shown only once.</p>
		<pre class="mt-3 overflow-x-auto rounded-lg bg-gray-950 p-4 text-xs text-gray-100"><code>Authorization: Bearer &lt;id&gt;|&lt;token&gt;
Accept: application/json</code></pre>
		<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">Base URL: <code class="font-mono">{base}</code>. Customers without API access receive HTTP 403.</p>
	</section>

	{#each endpoints as group (group.group)}
		<section class="space-y-3">
			<h2 class="text-lg font-bold text-gray-950 dark:text-white">{group.group}</h2>
			{#each group.items as endpoint (endpoint.method + endpoint.path)}
				<article class="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-800 dark:bg-gray-900">
					<div class="flex flex-wrap items-center gap-2">
						<span class="rounded-md border px-2 py-0.5 font-mono text-[11px] font-bold {methodClass(endpoint.method)}">{endpoint.method}</span>
						<code class="font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">{endpoint.path}</code>
					</div>
					<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{endpoint.description}</p>
					{#if endpoint.body}
						<p class="mt-3 text-xs font-semibold uppercase tracking-wider text-gray-500">Request</p>
						<pre class="mt-1 overflow-x-auto rounded-lg bg-gray-950 p-3 text-xs text-gray-100"><code>{endpoint.body}</code></pre>
					{/if}
					{#if endpoint.response}
						<p class="mt-3 text-xs font-semibold uppercase tracking-wider text-gray-500">Response</p>
						<pre class="mt-1 overflow-x-auto rounded-lg bg-gray-950 p-3 text-xs text-gray-100"><code>{endpoint.response}</code></pre>
					{/if}
				</article>
			{/each}
		</section>
	{/each}
</div>
