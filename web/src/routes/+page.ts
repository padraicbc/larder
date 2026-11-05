import { api } from '#lib/api.ts';
import { addDays, todayISO } from '#lib/format.ts';
import type { DayLog, History, Recipe, LogEntry, Product } from '#lib/types.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, url }) => {
	const date = url.searchParams.get('date') ?? todayISO();
	// Center the week strip on the chosen day where possible, never past today.
	const end = [addDays(date, 3), todayISO()].sort()[0];
	const [day, history, recipes, recentLogs, products] = await Promise.all([
		api<DayLog>(`/log?date=${date}`, { fetch }),
		api<History>(`/log/history?days=7&end=${end}`, { fetch }),
		api<Recipe[]>('/recipes', { fetch }),
		api<LogEntry[]>('/log/recent', { fetch }),
		api<Product[]>('/products', { fetch })
	]);
	return { date, day, history, recipes, recentLogs, products };
};
