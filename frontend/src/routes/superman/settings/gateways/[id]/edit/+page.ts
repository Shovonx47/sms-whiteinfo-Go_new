import { redirect } from '@sveltejs/kit';

export const ssr = false;

export function load({ params, url }) {
	redirect(307, `/superman/gateways/${params.id}/edit${url.search}`);
}
