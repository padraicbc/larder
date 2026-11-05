import { api } from '#lib/api.ts';
import type { PantryItem, PlanDetail, ShoppingList } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, params }) => {
	const [list, detail, pantry] = await Promise.all([
		api<ShoppingList>(`/plans/${params.id}/shopping`, { fetch }),
		api<PlanDetail>(`/plans/${params.id}`, { fetch }),
		api<PantryItem[]>('/pantry', { fetch })
	]);
	return { list, plan: detail.plan, pantryNames: pantry.map((p) => p.name) };
};
