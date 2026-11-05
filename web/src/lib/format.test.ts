import { describe, expect, it } from 'vitest';
import { addDays, friendlyDate, groupBy, inPantry, ratio, shoppingText, splitList, sumMacros, toISO } from './format';

describe('dates', () => {
	it('formats local dates without UTC drift', () => {
		expect(toISO(new Date(2026, 0, 5, 23, 59))).toBe('2026-01-05');
	});
	it('adds days across month and year boundaries', () => {
		expect(addDays('2026-01-31', 1)).toBe('2026-02-01');
		expect(addDays('2026-01-01', -1)).toBe('2025-12-31');
	});
	it('names nearby days', () => {
		expect(friendlyDate('2026-03-10', '2026-03-10')).toBe('Today');
		expect(friendlyDate('2026-03-09', '2026-03-10')).toBe('Yesterday');
		expect(friendlyDate('2026-03-11', '2026-03-10')).toBe('Tomorrow');
	});
});

describe('ratio', () => {
	it.each([
		[50, 100, 0.5],
		[150, 100, 1],
		[-5, 100, 0],
		[10, 0, 0]
	])('ratio(%d, %d) = %d', (v, t, want) => expect(ratio(v, t)).toBe(want));
});

it('sums macros', () => {
	const m = { calories: 100, protein: 10, carbs: 5, fat: 2, fiber: 1 };
	expect(sumMacros([m, m]).calories).toBe(200);
	expect(sumMacros([]).protein).toBe(0);
});

it('groups preserving first-seen order', () => {
	expect(groupBy([1, 2, 3, 4], (n) => (n % 2 ? 'odd' : 'even')).map(([k]) => k)).toEqual(['odd', 'even']);
});

it('builds shopping text from unchecked items only', () => {
	const text = shoppingText([
		{ key: 'a', name: 'lime', quantity: 2, unit: '', category: 'produce', checked: false, manual: false },
		{ key: 'b', name: 'rice', quantity: 500, unit: 'g', category: 'grains', checked: true, manual: false },
		{ key: 'c', name: 'coffee', quantity: 0, unit: '', category: 'other', checked: false, manual: true }
	]);
	expect(text).toBe('Produce\n- lime (2)\n\nOther\n- coffee');
});

describe('inPantry', () => {
	const pantry = ['feta', 'greek yogurt', 'cherry tomatoes', 'eggs'];
	it.each([
		['crumbled feta cheese', true],
		['yogurt', true],
		['Cherry Tomato', true],
		['eggplant', false],
		['tomato paste', false]
	])('%s -> %s', (ing, want) => expect(inPantry(ing, pantry)).toBe(want));
});

it('splits typed lists', () => {
	expect(splitList('eggs, spinach,\n feta ,, ')).toEqual(['eggs', 'spinach', 'feta']);
});
