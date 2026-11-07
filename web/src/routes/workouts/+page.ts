import { api } from '#lib/api.ts';
import type { Workout } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [workouts, kinds] = await Promise.all([
		api<Workout[]>('/workouts', { fetch }),
		api<{ kinds: string[]; builtIn: string[] }>('/workout-kinds', { fetch })
	]);
	return { workouts, kinds: kinds.kinds, builtIn: kinds.builtIn };
};
