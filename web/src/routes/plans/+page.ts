import { api } from '#lib/api.ts';
import type { MealPlan, Profile, SavedMeal } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [plans, profile, meals] = await Promise.all([
		api<MealPlan[]>('/plans', { fetch }),
		api<Profile>('/profile', { fetch }),
		api<SavedMeal[]>('/meals', { fetch })
	]);
	return { plans, profile, meals };
};
