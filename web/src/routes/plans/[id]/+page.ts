import { api } from '#lib/api.ts';
import type { PlanDetail, Profile } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const [detail, profile] = await Promise.all([
		api<PlanDetail>(`/plans/${params.id}`, { fetch }),
		api<Profile>('/profile', { fetch })
	]);
	return { ...detail, profile };
};
