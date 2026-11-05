import { api } from '#lib/api.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const status = await api<{ signupOpen: boolean; firstUser: boolean }>('/auth/status', { fetch });
	// Only follow same-site paths after sign-in (no open redirects).
	const next = url.searchParams.get('next') ?? '/';
	return { status, next: next.startsWith('/') && !next.startsWith('//') ? next : '/' };
};
