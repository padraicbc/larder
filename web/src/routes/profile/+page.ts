import { api } from '#lib/api.ts';
import type { Profile } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => ({ profile: await api<Profile>('/profile', { fetch }) });
