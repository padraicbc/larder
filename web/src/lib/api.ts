import { error } from '@sveltejs/kit';
import { goto } from '$app/navigation';

export class ApiError extends Error {
	constructor(
		public status: number,
		message: string
	) {
		super(message);
	}
}

type Fetch = typeof fetch;

async function handle<T>(res: Response, path: string, fromLoad = false): Promise<T> {
	if (res.status === 204) return undefined as T;
	const data = await res.json().catch(() => ({}));
	if (res.status === 401 && !path.startsWith('/auth/')) {
		// Session expired or missing: send them to sign in, then back here.
		const here = location.pathname + location.search;
		await goto(`/login?next=${encodeURIComponent(here)}`);
	}
	const message = data.error ?? `Request failed (${res.status})`;
	// Inside load functions, a missing plan/recipe becomes a real 404 page.
	if (fromLoad && res.status === 404) error(404, message);
	if (!res.ok) throw new ApiError(res.status, message);
	return data as T;
}

/**
 * Call the Go API. The session cookie is sent automatically (same origin).
 * Pass SvelteKit's `fetch` from load functions.
 */
export async function api<T = unknown>(
	path: string,
	opts: { method?: string; body?: unknown; fetch?: Fetch } = {}
): Promise<T> {
	const f = opts.fetch ?? fetch;
	const res = await f(`/api${path}`, {
		method: opts.method ?? (opts.body === undefined ? 'GET' : 'POST'),
		headers: opts.body === undefined ? undefined : { 'Content-Type': 'application/json' },
		body: opts.body === undefined ? undefined : JSON.stringify(opts.body)
	});
	return handle<T>(res, path, opts.fetch !== undefined);
}

/** POST multipart form data (photo uploads). */
export async function upload<T = unknown>(path: string, form: FormData): Promise<T> {
	const res = await fetch(`/api${path}`, { method: 'POST', body: form });
	return handle<T>(res, path);
}

/** Server messages start lowercase ("give the meal a name"); show them as sentences. */
export const sentence = (s: string) => (s ? s[0].toLocaleUpperCase() + s.slice(1) : s);

export const errorText = (e: unknown) =>
	e instanceof Error ? sentence(e.message) : 'Something went wrong. Check that the Go server is running.';
