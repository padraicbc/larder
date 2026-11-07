import { api } from '#lib/api.ts';
import type { SavedMeal } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({ meals: await api<SavedMeal[]>('/meals', { fetch }) });
