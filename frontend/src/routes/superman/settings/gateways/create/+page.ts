import { redirect } from '@sveltejs/kit';

export const ssr = false;

export function load({ url }) {
	redirect(307, `/superman/gateways/create${url.search}`);
}
