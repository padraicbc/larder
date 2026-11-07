import { describe, expect, it } from 'vitest';
import { dateTicks, inRange, niceTicks, rangeStart, summarize } from './weight-chart';

const e = (date: string, kg: number) => ({ date, kg });

describe('weight chart helpers', () => {
	it('goes back whole calendar months', () => {
		expect(rangeStart('2m', '2026-03-10', [])).toBe('2026-01-10');
		expect(rangeStart('6m', '2026-03-10', [])).toBe('2025-09-10');
		expect(rangeStart('2y', '2026-03-10', [])).toBe('2024-03-10');
		expect(rangeStart('all', '2026-03-10', [e('2025-06-01', 80)])).toBe('2025-06-01');
		expect(rangeStart('all', '2026-03-10', [])).toBe('2026-03-10');
	});
	it('filters inclusively', () => {
		const list = [e('2026-01-09', 1), e('2026-01-10', 2), e('2026-03-10', 3), e('2026-03-11', 4)];
		expect(inRange(list, '2026-01-10', '2026-03-10').map((x) => x.kg)).toEqual([2, 3]);
	});
	it('picks round gridlines that cover the data', () => {
		expect(niceTicks(81.2, 85.4)).toEqual([80, 82, 84, 86]);
		expect(niceTicks(80, 80)[0]).toBeLessThanOrEqual(80);
		const t = niceTicks(60.3, 95);
		expect(t[0]).toBeLessThanOrEqual(60.3);
		expect(t[t.length - 1]).toBeGreaterThanOrEqual(95);
	});
	it('spaces date labels across the window', () => {
		const t = dateTicks('2026-01-01', '2026-01-31', 3);
		expect(t).toEqual(['2026-01-01', '2026-01-11', '2026-01-21', '2026-01-31']);
		expect(dateTicks('2026-01-01', '2026-01-01')).toEqual(['2026-01-01']);
	});
	it('summarises change over the period', () => {
		expect(summarize([])).toBeNull();
		const s = summarize([e('2026-01-01', 85), e('2026-02-01', 83.2), e('2026-03-01', 83.9)])!;
		expect(s.change).toBe(-1.1);
		expect(s.min).toBe(83.2);
		expect(s.max).toBe(85);
	});
});
