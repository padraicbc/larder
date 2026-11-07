import { toISO } from './format';
import type { WeightEntry } from './types';

export type Range = '2m' | '6m' | '1y' | '2y' | 'all';
export const RANGES: { id: Range; label: string }[] = [
	{ id: '2m', label: '2 months' },
	{ id: '6m', label: '6 months' },
	{ id: '1y', label: '1 year' },
	{ id: '2y', label: '2 years' },
	{ id: 'all', label: 'All' }
];

const MONTHS: Record<Exclude<Range, 'all'>, number> = { '2m': 2, '6m': 6, '1y': 12, '2y': 24 };

const parse = (iso: string) => {
	const [y, m, d] = iso.split('-').map(Number);
	return new Date(y, m - 1, d);
};
/** Whole days since the epoch for a local date (DST-safe, unlike ms maths). */
export const dayNumber = (iso: string) => {
	const [y, m, d] = iso.split('-').map(Number);
	return Math.round(Date.UTC(y, m - 1, d) / 86_400_000);
};

/** First day of the window: N calendar months back, or the first reading for "all". */
export function rangeStart(range: Range, today: string, entries: WeightEntry[]): string {
	if (range === 'all') return entries[0]?.date ?? today;
	const d = parse(today);
	d.setMonth(d.getMonth() - MONTHS[range]);
	return toISO(d);
}

export function inRange(entries: WeightEntry[], start: string, end: string): WeightEntry[] {
	return entries.filter((e) => e.date >= start && e.date <= end);
}

/** Round-number gridlines that cover [min, max] with roughly `count` steps. */
export function niceTicks(min: number, max: number, count = 4): number[] {
	const span = Math.max(max - min, 1);
	const raw = span / count;
	const step = [0.5, 1, 2, 5, 10, 20].find((s) => s >= raw) ?? 20;
	const lo = Math.floor(min / step) * step;
	const hi = Math.ceil(max / step) * step;
	const out: number[] = [];
	for (let v = lo; v <= hi + 1e-9; v += step) out.push(Math.round(v * 10) / 10);
	return out;
}

/** Evenly spaced date labels across the window (start and end included). */
export function dateTicks(start: string, end: string, count = 4): string[] {
	const a = dayNumber(start);
	const b = dayNumber(end);
	if (b <= a) return [start];
	return Array.from({ length: count + 1 }, (_, i) => {
		const n = a + Math.round(((b - a) * i) / count);
		const d = new Date(n * 86_400_000);
		return toISO(new Date(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));
	});
}

export function summarize(visible: WeightEntry[]) {
	if (!visible.length) return null;
	const kgs = visible.map((e) => e.kg);
	const first = visible[0];
	const last = visible[visible.length - 1];
	return { first, last, change: Math.round((last.kg - first.kg) * 10) / 10, min: Math.min(...kgs), max: Math.max(...kgs) };
}
