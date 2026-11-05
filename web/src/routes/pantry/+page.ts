import { api } from '#lib/api.ts';
import type { PantryItem } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({ items: await api<PantryItem[]>('/pantry', { fetch }) });
