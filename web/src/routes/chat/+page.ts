import { api } from '#lib/api.ts';
import type { PageLoad } from './$types';
export const load: PageLoad = async ({ fetch }) => ({ chat: await api<{model: string; canSearch: boolean; configured: boolean}>('/food-chat', {fetch}) });
