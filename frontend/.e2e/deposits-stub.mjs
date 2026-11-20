import http from 'node:http';

const customers = [
	'Md Abdullah',
	'Rima Parvin',
	'Firoz Mahmud Rafi',
	'White Info Test',
	'Md Lutfor Rahman Rafe',
	'Md Mokhlasur Rahman'
];
const companies = ['As Embroidery', 'Interaid', 'Friends Recruitment World', "Tailor's Craft", 'Bright Academy', 'White Info'];
const statuses = ['completed', 'completed', 'pending', 'failed'];

const deposits = Array.from({ length: 23 }, (_, index) => {
	const n = index + 1;
	const status = statuses[index % statuses.length];
	const method = index % 3 === 0 ? 'manual' : 'piprapay';
	const day = 18 - Math.floor(index / 2);
	return {
		id: 200 + n,
		user_id: n,
		customer: customers[index % customers.length],
		customer_email: `user${n}@example.com`,
		customer_phone: `+88017${String(10000000 + n)}`,
		company: companies[index % companies.length],
		amount: [100, 500, 1000, 300, 50][index % 5],
		currency: 'BDT',
		payment_method: method,
		gateway: method,
		transaction_id: method === 'piprapay' ? `3850310623096688131528${String(n).padStart(4, '0')}` : null,
		deposited_by: status === 'completed' ? (method === 'manual' ? 'Superman' : 'PipraPay') : null,
		status,
		paid_at: status === 'completed' ? new Date(Date.UTC(2026, 10, Math.max(day, 1), 12, n)).toISOString() : null,
		created_at: new Date(Date.UTC(2026, 10, Math.max(day, 1), 9, n)).toISOString()
	};
});

// Rebuilt per request so mutations (approve, reject, edit) show up in the
// finance report exactly as they would from the Go aggregate queries.
function buildReport(period = 'today') {
	const days = period === 'today' || period === 'yesterday' ? 1 : 14;

	const daily = Array.from({ length: days }, (_, index) => {
		const rows = deposits.filter((d) => new Date(d.created_at).getUTCDate() === 18 - index);
		const add = rows.filter((d) => d.status === 'completed').reduce((sum, d) => sum + d.amount, 0);
		const minus = rows.filter((d) => d.status === 'failed' || d.status === 'rejected').reduce((sum, d) => sum + d.amount, 0);
		return {
			date: new Date(Date.UTC(2026, 10, 18 - index)).toISOString().slice(0, 10),
			add,
			minus,
			net: add - minus,
			transactions: rows.filter((d) => d.status === 'completed').length
		};
	});

	const totals = daily.reduce(
		(acc, day) => ({
			add: acc.add + day.add,
			minus: acc.minus + day.minus,
			transactions: acc.transactions + day.transactions
		}),
		{ add: 0, minus: 0, transactions: 0 }
	);
	totals.net = totals.add - totals.minus;

	const scoped = period === 'today' || period === 'yesterday' ? deposits.filter((d) => new Date(d.created_at).getUTCDate() === 18) : deposits;
	const channels = [
		{
			key: 'ADMIN',
			label: 'ADMIN',
			add: scoped.filter((d) => d.payment_method === 'manual' && d.status === 'completed').reduce((s, d) => s + d.amount, 0),
			minus: scoped.filter((d) => d.payment_method === 'manual' && (d.status === 'failed' || d.status === 'rejected')).reduce((s, d) => s + d.amount, 0)
		},
		{
			key: 'PIPRAPAY',
			label: 'PIPRAPAY',
			add: scoped.filter((d) => d.payment_method === 'piprapay' && d.status === 'completed').reduce((s, d) => s + d.amount, 0),
			minus: scoped.filter((d) => d.payment_method === 'piprapay' && (d.status === 'failed' || d.status === 'rejected')).reduce((s, d) => s + d.amount, 0)
		}
	];

	return { totals, channels, daily };
}

const server = http.createServer((req, res) => {
	const url = new URL(req.url, 'http://localhost:8081');
	const send = (payload, status = 200) => {
		res.writeHead(status, { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*' });
		res.end(JSON.stringify(payload));
	};

	console.log(req.method, url.pathname + url.search);

	if (url.pathname === '/api/v1/superman/auth/me') {
		return send({ data: { id: 1, name: 'Superman', email: 'admin@whiteinfobd.com', type: 'superman' } });
	}
	if (url.pathname === '/api/v1/superman/topbar-stats') {
		return send({ data: {} });
	}
	if (url.pathname === '/api/v1/superman/deposits/finance-report') {
		const period = url.searchParams.get('period') || 'today';
		const report = buildReport(period);
		return send({
			data: {
				period,
				from: url.searchParams.get('from') || '2026-11-05',
				to: url.searchParams.get('to') || '2026-11-18',
				...report
			}
		});
	}

	const postMatch = url.pathname.match(/^\/api\/v1\/superman\/deposits\/(\d+)\/(approve|reject)$/);
	if (postMatch && req.method === 'POST') {
		const target = deposits.find((d) => d.id === Number(postMatch[1]));
		if (!target) return send({ message: 'Deposit not found.' }, 404);
		target.status = postMatch[2] === 'approve' ? 'completed' : 'rejected';
		target.deposited_by = 'Superman';
		target.paid_at = new Date().toISOString();
		return send({ message: `Deposit ${postMatch[2]}d.` });
	}

	const detailIdMatch = url.pathname.match(/^\/api\/v1\/superman\/deposits\/(\d+)$/);
	if (detailIdMatch && req.method === 'PUT') {
		const target = deposits.find((d) => d.id === Number(detailIdMatch[1]));
		if (!target) return send({ message: 'Deposit not found.' }, 404);

		let raw = '';
		req.on('data', (chunk) => (raw += chunk));
		req.on('end', () => {
			let body = {};
			try {
				body = raw ? JSON.parse(raw) : {};
			} catch {
				body = {};
			}

			if (target.status === 'completed' && body.status && body.status !== 'completed') {
				return send({ message: 'An approved deposit cannot be reopened.' }, 400);
			}

			const becomingCompleted = target.status !== 'completed' && body.status === 'completed';
			if (typeof body.amount === 'number') target.amount = body.amount;
			if (typeof body.currency === 'string' && body.currency) target.currency = body.currency.toUpperCase();
			if (typeof body.payment_method === 'string' && body.payment_method) {
				target.payment_method = body.payment_method;
				target.gateway = body.payment_method;
			}
			if ('transaction_id' in body) target.transaction_id = body.transaction_id || null;
			if (typeof body.status === 'string') target.status = body.status;
			if ('paid_at' in body) target.paid_at = body.paid_at ? new Date(body.paid_at).toISOString() : null;
			if (becomingCompleted) {
				target.deposited_by = 'Superman';
				if (!target.paid_at) target.paid_at = new Date().toISOString();
			}

			send({ message: `Deposit #${target.id} updated.`, data: target });
		});
		return;
	}

	const detailMatch = url.pathname.match(/^\/api\/v1\/superman\/deposits\/(\d+)$/);
	if (detailMatch) {
		const target = deposits.find((d) => d.id === Number(detailMatch[1]));
		if (!target) return send({ message: 'Deposit not found.' }, 404);
		return send({
			data: {
				...target,
				gateway_name: 'piprapay',
				gateway_response:
					target.transaction_id === null
						? ''
						: JSON.stringify({
								pp_id: target.transaction_id,
								pp_url: `https://pay.whiteinfobd.com/payment/${target.transaction_id}`,
								status: target.status
							})
			}
		});
	}

	if (url.pathname === '/api/v1/superman/deposits') {
		const page = Number(url.searchParams.get('page') || '1');
		const perPage = Number(url.searchParams.get('per_page') || '10');
		const status = url.searchParams.get('status');
		const method = url.searchParams.get('method');
		const search = (url.searchParams.get('search') || '').toLowerCase();

		let rows = deposits;
		if (status && status !== 'all') rows = rows.filter((d) => d.status === status);
		if (method && method !== 'all') rows = rows.filter((d) => d.payment_method === method);
		if (search) {
			rows = rows.filter(
				(d) =>
					d.customer.toLowerCase().includes(search) ||
					d.customer_phone.includes(search) ||
					(d.transaction_id || '').includes(search)
			);
		}

		const start = (page - 1) * perPage;
		return send({
			data: rows.slice(start, start + perPage),
			meta: { page, per_page: perPage, total: rows.length }
		});
	}

	return send({ data: [] });
});

server.listen(8081, '127.0.0.1', () => console.log('stub listening on 8081'));
