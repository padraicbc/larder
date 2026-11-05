import { api } from '#lib/api.ts';
import type { Rating, Recipe } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) =>
	api<{ recipe: Recipe; ratings: Rating[] }>(`/recipes/${params.id}`, { fetch });
