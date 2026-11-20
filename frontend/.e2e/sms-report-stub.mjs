// Local fixture API for the Superman SMS Reports page.
// Mirrors /api/v1/superman/messages so the page can be exercised without MySQL.
import { createServer } from 'node:http';

const users = ['MD Nasir Hasan', 'Arifuz Zaman', 'MD TASRIF HOSSAIN', 'Sabbir Ahmed'];
const recipients = [
	'+8801326221808', '+8801826513091', '+8801925942222', '+8801909698479', '+8801711223344', '+8801812345678'
];
const bodies = [
	'Your Advance Earning verification OTP is: 618542',
	'Your Advance Earning verification OTP is: 140063',
	'Congratulations H M Akash Ahammmed! Your registration is complete.',
	'Welcome to ToTo Food. Your login OTP is 918263. Do not share this code.',
	'Your Advance Earning verification OTP is: 355281'
];
const operators = ['Grameenphone', 'Robi', 'Banglalink'];
const senders = ['8809639228876', '8809639228925', '8809639228870'];
const gateways = ['Infozillion IPTSP'];
const statuses = ['delivered', 'delivered', 'delivered', 'sent', 'sent', 'failed', 'pending'];

function pad(value, size = 6) {
	return String(value).padStart(size, '0');
}

function uuid(seed, offset) {
	const base = (seed * 7919 + offset * 104729).toString(16).padStart(12, '0');
	return `${base.slice(0, 8)}-${base.slice(0, 4)}-4${base.slice(1, 4)}-9${base.slice(2, 5)}-${base.padEnd(12, 'a').slice(0, 12)}`;
}

function iso(date) {
	return date.toISOString();
}

function buildRows() {
	const rows = [];
	const start = new Date('2026-11-19T23:18:09+06:00');
	let balance = 28.25;
	for (let index = 0; index < 164; index++) {
		const submitted = new Date(start.getTime() - index * 37_000);
		const status = statuses[index % statuses.length];
		const delivered =
			status === 'delivered' ? new Date(submitted.getTime() + Math.floor(53_000 + index * 900)) : null;
		const operator = operators[index % operators.length];
		const sender = senders[index % senders.length];
		rows.push({
			id: 900164 - index,
			customer: users[index % users.length],
			recipient: recipients[index % recipients.length],
			body: bodies[index % bodies.length],
			status,
			cost: 0.25,
			operator_cost: 0.1,
			primary_dipping_cost: 0,
			parts: 1,
			source: index % 5 === 3 ? 'api' : 'web',
			gateway: gateways[0],
			gateway_driver: 'infozillion',
			sender_id: sender,
			sender_type: 'numeric',
			bill_msisdn: sender,
			sms_type: 'T',
			channel: 'IPTSP',
			operator,
			tariff: 'Offer-Non-25 Mask-55',
			api_user: index % 5 === 3 ? users[index % users.length] : null,
			source_ip: index % 4 === 0 ? '151.158.158.199' : '103.148.14.50',
			request_id: uuid(index + 3, 1),
			server_txn_id: uuid(index + 5, 2),
			server_response_code: '9000',
			mno_txn_id: `ICC-2026111923${pad(1809 + index, 4)}-${recipients[index % recipients.length].slice(1)}-${pad(
				4533787583 + index,
				10
			)}`,
			mno_response_code: status === 'failed' ? '1001' : '1000',
			mno_response_message: status === 'failed' ? 'Invalid MSISDN' : 'Success',
			previous_balance: (balance + 0.25).toFixed(4),
			next_balance: balance.toFixed(4),
			failure_reason: status === 'failed' ? 'SMS delivery status: Undelivered' : '',
			submit_time: iso(submitted),
			delivery_time: delivered ? iso(delivered) : null,
			created_at: iso(submitted)
		});
		balance += 0.25;
	}
	return rows;
}

const rows = buildRows();

const server = createServer((request, response) => {
	const url = new URL(request.url ?? '/', 'http://127.0.0.1:8081');
	response.setHeader('Access-Control-Allow-Origin', '*');
	response.setHeader('Access-Control-Allow-Headers', '*');
	response.setHeader('Content-Type', 'application/json');

	if (url.pathname === '/api/v1/superman/auth/me') {
		response.end(
			JSON.stringify({ data: { id: 1, name: 'E2E Superman', email: 'superman@test.local', type: 'superman' } })
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/sps') {
		response.end(
			JSON.stringify({
				data: [
					{
						id: 1,
						name: 'Softcents Telecom',
						email: 'sp@softcents.com',
						phone: '+8801700000011',
						business_name: 'Softcents Ltd.',
						balance: 125000.5,
						customer_count: 42,
						commission_distributed: 8600.25,
						status: 'approved'
					},
					{
						id: 2,
						name: 'Greenline Resellers',
						email: 'ops@greenline.bd',
						phone: '+8801800000022',
						business_name: null,
						balance: 4300,
						customer_count: 7,
						commission_distributed: 615.75,
						status: 'approved'
					}
				],
				meta: { page: 1, per_page: 10, total: 2 }
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/customers') {
		response.end(
			JSON.stringify({
				data: [
					{
						id: 11,
						name: 'MD Nasir Hasan',
						email: 'nasir@example.com',
						phone: '+8801326221808',
						balance: 28.25,
						messages_count: 612,
						messages_spend: 153.4,
						created_at: '2026-08-14T10:20:00+06:00'
					},
					{
						id: 12,
						name: 'Arifuz Zaman',
						email: 'arifuz@example.com',
						phone: '+8801826513091',
						balance: 420.75,
						messages_count: 480,
						messages_spend: 96.15,
						created_at: '2026-09-02T14:05:00+06:00'
					}
				],
				meta: { page: 1, per_page: 10, total: 2 }
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/providers/health') {
		response.end(
			JSON.stringify({
				data: {
					gateways: [
						{ id: 3, name: 'Infozillion IPTSP', driver: 'infozillion', health_status: 'healthy', is_enabled: true, total_dispatches: 5210, revenue: 1302.5 },
						{ id: 4, name: 'GP MNO Direct', driver: 'infozillion', health_status: 'degraded', is_enabled: true, total_dispatches: 880, revenue: 220 },
						{ id: 5, name: 'Legacy Log Driver', driver: 'log', health_status: 'down', is_enabled: false, total_dispatches: 0, revenue: 0 }
					]
				}
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/billing/reports') {
		response.end(
			JSON.stringify({
				data: [
					{ id: 91, user_id: 11, customer_name: 'MD Nasir Hasan', customer_phone: '+8801326221808', reference: 'message_batch', direction: 'debit', amount: 5.5, balance_before: 28.5, balance_after: 23, description: 'SMS batch debit (11 messages)', created_at: '19 Nov 2026, 11:18 PM' },
					{ id: 92, user_id: 12, customer_name: 'Arifuz Zaman', customer_phone: '+8801826513091', reference: 'deposit', direction: 'credit', amount: 500, balance_before: 0, balance_after: 500, description: 'Manual deposit approved by admin', created_at: '19 Nov 2026, 09:12 PM' }
				],
				meta: { page: 1, per_page: 10, total: 2 },
				summary: { total_credits: 500, total_debits: 5.5, net_flow: 494.5, total_tx_count: 2 }
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/kyc/dashboard') {
		const kycCustomers = [
			{ id: 102, full_name: 'Mehedi Hasan', email: 'mehedihasan01933358@gmail.com', mobile: '+8801933358507', nid_number: '2357731070', kyc_status: 'verified', submitted_at: null, verified_at: '2026-11-10T10:00:00+06:00' },
			{ id: 101, full_name: 'Arifur Rahman Bhuiyan', email: 'arif.seu50@gmail.com', mobile: '+8801685955488', nid_number: null, kyc_status: 'verified', submitted_at: null, verified_at: '2026-11-09T10:00:00+06:00' },
			{ id: 100, full_name: 'Md Raju Amin', email: 'interaidbagerhat@gmail.com', mobile: '+8801999787980', nid_number: null, kyc_status: 'verified', submitted_at: null, verified_at: '2026-11-08T10:00:00+06:00' },
			{ id: 99, full_name: 'ABDULLAH AL MAHADI', email: 'mahdihasan8800@gmail.com', mobile: '+8801879216698', nid_number: null, kyc_status: 'verified', submitted_at: null, verified_at: '2026-11-07T10:00:00+06:00' },
			{ id: 98, full_name: 'Super Admin', email: 'admin@whiteinfobd.com', mobile: '+8801700000000', nid_number: null, kyc_status: 'verified', submitted_at: null, verified_at: '2026-11-06T10:00:00+06:00' },
			{ id: 97, full_name: 'Sadia White Info', email: 'smostarin10@gmail.com', mobile: '+8801571387100', nid_number: null, kyc_status: 'verified', submitted_at: null, verified_at: '2026-11-05T10:00:00+06:00' },
			{ id: 96, full_name: 'Dr. Arafat', email: 'amctik@gmail.com', mobile: '+8801911313862', nid_number: null, kyc_status: 'verified', submitted_at: null, verified_at: '2026-11-04T10:00:00+06:00' },
			{ id: 95, full_name: 'Tanvir Hossain', email: 'tanvir@example.com', mobile: '+8801711223344', nid_number: '1999887766', kyc_status: 'under_review', submitted_at: '2026-11-18T12:30:00+06:00', verified_at: null }
		];
		const search = (url.searchParams.get('search') ?? '').trim().toLowerCase();
		const status = (url.searchParams.get('status') ?? '').trim();
		const page = Math.max(1, Number(url.searchParams.get('page') ?? 1));
		const perPage = Math.min(200, Math.max(1, Number(url.searchParams.get('per_page') ?? 10)));
		const filtered = kycCustomers.filter((customer) => {
			if (status && status !== 'all' && customer.kyc_status !== status) return false;
			if (!search) return true;
			return [customer.full_name, customer.email, customer.mobile]
				.some((value) => (value ?? '').toLowerCase().includes(search));
		});
		response.end(
			JSON.stringify({
				stats: {
					total_kyc: 79,
					pending: 0,
					under_review: 0,
					verified: 79,
					rejected: 0,
					resubmit_required: 0,
					today_submissions: 0,
					today_approved: 0
				},
				data: filtered.slice((page - 1) * perPage, page * perPage),
				meta: { page, per_page: perPage, total: filtered.length, total_pages: Math.max(1, Math.ceil(filtered.length / perPage)) }
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/contact-owners') {
		response.end(
			JSON.stringify({
				data: [
					{ id: 102, name: 'Mehedi Hasan' },
					{ id: 101, name: 'Arifur Rahman Bhuiyan' },
					{ id: 100, name: 'Md Raju Amin' }
				]
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/contacts') {
		if (request.method !== 'GET') {
			response.statusCode = 201;
			response.end(JSON.stringify({ message: 'Contact saved.', data: { id: 999 } }));
			return;
		}

		const contacts = [
			{ id: 1, user_id: 102, user_name: 'Mehedi Hasan', name: 'Rakib', phone: '+8801711223344', email: null, group_name: 'VIP Priority Customers', meta: null, created_at: '2026-11-10T10:00:00+06:00', updated_at: '2026-11-10T10:00:00+06:00', deleted_at: null },
			{ id: 2, user_id: 101, user_name: 'Arifur Rahman Bhuiyan', name: 'Sadia', phone: '+8801812345678', email: null, group_name: 'Marketing & Promo Pool', meta: { source: 'CSV import' }, created_at: '2026-11-09T10:00:00+06:00', updated_at: null, deleted_at: null }
		];
		const contactSearch = (url.searchParams.get('search') ?? '').trim().toLowerCase();
		const filteredContacts = contactSearch
			? contacts.filter((row) => [row.user_name, row.name, row.phone].join(' ').toLowerCase().includes(contactSearch))
			: contacts;
		const contactPage = Math.max(1, Number(url.searchParams.get('page') ?? 1));
		const contactPerPage = Math.max(1, Number(url.searchParams.get('per_page') ?? 10));
		response.end(
			JSON.stringify({
				data: filteredContacts.slice((contactPage - 1) * contactPerPage, contactPage * contactPerPage),
				meta: { page: contactPage, per_page: contactPerPage, total: filteredContacts.length }
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/contacts/bulk') {
		response.end(JSON.stringify({ message: 'Bulk action applied.', affected: 1 }));
		return;
	}

	if (url.pathname.startsWith('/api/v1/superman/contacts/')) {
		response.end(JSON.stringify({ message: request.method === 'DELETE' ? 'Contact deleted.' : 'Contact updated.' }));
		return;
	}

	if (url.pathname === '/api/v1/superman/contact-groups') {
		if (request.method !== 'GET') {
			response.statusCode = 201;
			response.end(JSON.stringify({ message: 'Contact group saved.', data: { id: 999 } }));
			return;
		}

		const groups = [
			{ id: 1, user_id: 102, user_name: 'Mehedi Hasan', name: 'VIP Priority Customers', description: 'High value numbers', contacts_count: 12, created_at: '2026-11-10T10:00:00+06:00', updated_at: null, deleted_at: null },
			{ id: 2, user_id: 101, user_name: 'Arifur Rahman Bhuiyan', name: 'Marketing & Promo Pool', description: null, contacts_count: 40, created_at: '2026-11-09T10:00:00+06:00', updated_at: null, deleted_at: null }
		];
		const groupSearch = (url.searchParams.get('search') ?? '').trim().toLowerCase();
		const filteredGroups = groupSearch
			? groups.filter((row) => [row.user_name, row.name].join(' ').toLowerCase().includes(groupSearch))
			: groups;
		const groupPage = Math.max(1, Number(url.searchParams.get('page') ?? 1));
		const groupPerPage = Math.max(1, Number(url.searchParams.get('per_page') ?? 10));
		response.end(
			JSON.stringify({
				data: filteredGroups.slice((groupPage - 1) * groupPerPage, groupPage * groupPerPage),
				meta: { page: groupPage, per_page: groupPerPage, total: filteredGroups.length }
			})
		);
		return;
	}

	if (url.pathname === '/api/v1/superman/contact-groups/bulk') {
		response.end(JSON.stringify({ message: 'Bulk action applied.', affected: 1 }));
		return;
	}

	if (url.pathname.startsWith('/api/v1/superman/contact-groups/')) {
		response.end(JSON.stringify({ message: request.method === 'DELETE' ? 'Contact group deleted.' : 'Contact group updated.' }));
		return;
	}

	if (url.pathname.startsWith('/api/v1/superman/kyc/requests/')) {
		response.end(
			JSON.stringify({
				data: {
					front_nid: null,
					back_nid: null,
					live_photo: null,
					trade_license_file: null
				}
			})
		);
		return;
	}

	if (url.pathname !== '/api/v1/superman/messages') {
		response.statusCode = 404;
		response.end(JSON.stringify({ message: 'Not found.' }));
		return;
	}

	const search = (url.searchParams.get('search') ?? '').trim().toLowerCase();
	const status = (url.searchParams.get('status') ?? '').trim();
	const statuses = (url.searchParams.get('statuses') ?? '').trim();
	const source = (url.searchParams.get('source') ?? '').trim();
	const page = Math.max(1, Number(url.searchParams.get('page') ?? 1));
	const perPage = Math.min(20000, Math.max(1, Number(url.searchParams.get('per_page') ?? 10)));
	const exporting = url.searchParams.get('export') === '1';

	let filtered = rows;
	if (search) {
		filtered = filtered.filter((row) =>
			[row.customer, row.recipient, row.body, row.sender_id, row.server_txn_id, row.tariff]
				.filter(Boolean)
				.some((value) => String(value).toLowerCase().includes(search))
		);
	}
	if (status) filtered = filtered.filter((row) => row.status === status);
	if (statuses) {
		const wanted = statuses.split(',').map((value) => value.trim()).filter(Boolean);
		filtered = filtered.filter((row) => wanted.includes(row.status));
	}
	if (source) filtered = filtered.filter((row) => row.source === source);
	const dateFrom = url.searchParams.get('date_from');
	const dateTo = url.searchParams.get('date_to');
	if (dateFrom) filtered = filtered.filter((row) => row.submit_time.slice(0, 10) >= dateFrom);
	if (dateTo) filtered = filtered.filter((row) => row.submit_time.slice(0, 10) <= dateTo);

	const start = exporting ? 0 : (page - 1) * perPage;
	const data = exporting ? filtered : filtered.slice(start, start + perPage);

	response.end(
		JSON.stringify({
			data,
			meta: { page: exporting ? 1 : page, per_page: exporting ? filtered.length : perPage, total: filtered.length }
		})
	);
});

server.listen(8081, '127.0.0.1', () => {
	console.log('SMS report stub listening on http://127.0.0.1:8081 with', rows.length, 'rows');
});
