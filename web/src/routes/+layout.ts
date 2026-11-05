import { redirect } from '@sveltejs/kit';
import type { User } from '#lib/types.ts';
import type { LayoutLoad } from './$types';

// Pure client-side app: the Go server owns all data.
export const ssr = false;
export const prerender = false;

export const load: LayoutLoad = async ({ fetch, url, depends }) => {
	depends('app:user');
	if (url.pathname === '/login') return { user: null };
	let res: Response;
	try {
		res = await fetch('/api/auth/me');
	} catch {
		// Offline: still render the shell so the error page can say so.
		return { user: null };
	}
	if (res.status === 401) {
		redirect(307, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
	}
	return { user: (await res.json()) as User };
};
