import { api } from '#lib/api.ts';
import type { WeightEntry } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({ entries: await api<WeightEntry[]>('/weight', { fetch }) });
