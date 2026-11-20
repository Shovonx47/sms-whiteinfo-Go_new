import { redirect } from '@sveltejs/kit';

export const ssr = false;

export function load({ params, url }) {
	redirect(307, `/superman/sender-cli/add/${params.id}${url.search}`);
}
