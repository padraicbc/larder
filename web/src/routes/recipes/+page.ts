import { api } from '#lib/api.ts';
import type { Feedback, Recipe } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [recipes, feedback] = await Promise.all([
		api<Recipe[]>('/recipes', { fetch }),
		api<Feedback>('/feedback', { fetch })
	]);
	return { recipes, feedback };
};
