import type { Macros, ShoppingItem } from './types';

const pad = (n: number) => String(n).padStart(2, '0');

/** Local calendar date as YYYY-MM-DD (not UTC — late-night snacks count for today). */
export function toISO(d: Date): string {
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export const todayISO = () => toISO(new Date());

export function addDays(iso: string, n: number): string {
	const [y, m, d] = iso.split('-').map(Number);
	return toISO(new Date(y, m - 1, d + n));
}

export function friendlyDate(iso: string, today = todayISO()): string {
	if (iso === today) return 'Today';
	if (iso === addDays(today, -1)) return 'Yesterday';
	if (iso === addDays(today, 1)) return 'Tomorrow';
	const [y, m, d] = iso.split('-').map(Number);
	return new Date(y, m - 1, d).toLocaleDateString(undefined, { weekday: 'short', day: 'numeric', month: 'short' });
}

/** value/target clamped to 0..1; 0 when there's no target. */
export function ratio(value: number, target: number): number {
	if (!target || target <= 0) return 0;
	return Math.min(Math.max(value / target, 0), 1);
}

export function round(n: number, places = 0): number {
	const p = 10 ** places;
	return Math.round(n * p) / p;
}

export function fmtQty(q: number, unit: string): string {
	const n = round(q, 2);
	return unit ? `${n} ${unit}` : `${n}`;
}

export function sumMacros(list: Macros[]): Macros {
	return list.reduce(
		(a, m) => ({
			calories: a.calories + m.calories,
			protein: a.protein + m.protein,
			carbs: a.carbs + m.carbs,
			fat: a.fat + m.fat,
			fiber: a.fiber + m.fiber
		}),
		{ calories: 0, protein: 0, carbs: 0, fat: 0, fiber: 0 }
	);
}

export function groupBy<T>(items: T[], key: (t: T) => string): [string, T[]][] {
	const map = new Map<string, T[]>();
	for (const it of items) {
		const k = key(it);
		if (!map.has(k)) map.set(k, []);
		map.get(k)!.push(it);
	}
	return [...map.entries()];
}

/** Plain-text list of what's still to buy, for pasting into notes/messages. */
export function shoppingText(items: ShoppingItem[]): string {
	return groupBy(
		items.filter((i) => !i.checked),
		(i) => i.category
	)
		.map(([cat, list]) => `${cap(cat)}\n${list.map((i) => `- ${i.name}${i.quantity ? ` (${fmtQty(i.quantity, i.unit)})` : ''}`).join('\n')}`)
		.join('\n\n');
}

export const cap = (s: string) => (s ? s[0].toUpperCase() + s.slice(1) : s);

// ---- pantry matching (mirrors backend nutrition.MatchPantry) ----

export function singular(s: string): string {
	s = s.toLowerCase().trim();
	if (s.endsWith('ies') && s.length > 4) return s.slice(0, -3) + 'y';
	if (s.endsWith('oes') && s.length > 4) return s.slice(0, -2);
	if (s.endsWith('ss') || s.endsWith('us')) return s;
	if (s.endsWith('s') && s.length > 3) return s.slice(0, -1);
	return s;
}

const words = (s: string) =>
	s
		.toLowerCase()
		.replace(/[^a-z0-9 ]+/g, ' ')
		.split(/\s+/)
		.filter(Boolean)
		.map(singular);

function containsWords(hay: string[], needle: string[]): boolean {
	if (!needle.length || needle.length > hay.length) return false;
	for (let i = 0; i + needle.length <= hay.length; i++) {
		if (needle.every((w, j) => hay[i + j] === w)) return true;
	}
	return false;
}

/** True when a pantry item covers this ingredient ("feta" covers "crumbled feta"). */
export function inPantry(ingredient: string, pantryNames: string[]): boolean {
	const iw = words(ingredient);
	return pantryNames.some((p) => {
		const pw = words(p);
		return containsWords(iw, pw) || containsWords(pw, iw);
	});
}

/** Split a comma/newline separated list typed by the user. */
export function splitList(text: string): string[] {
	return text
		.split(/[,\n]/)
		.map((s) => s.trim())
		.filter(Boolean);
}

/** "Thursday 5 November" (locale-aware). */
export function longDate(iso: string): string {
	const [y, m, d] = iso.split('-').map(Number);
	return new Date(y, m - 1, d).toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long' });
}
