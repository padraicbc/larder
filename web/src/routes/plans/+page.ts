import { api } from '#lib/api.ts';
import type { MealPlan, Profile } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [plans, profile] = await Promise.all([
		api<MealPlan[]>('/plans', { fetch }),
		api<Profile>('/profile', { fetch })
	]);
	return { plans, profile };
};
