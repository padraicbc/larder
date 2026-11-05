import { api } from '#lib/api.ts';
import type { Settings } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({ settings: await api<Settings>('/settings', { fetch }) });
