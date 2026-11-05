import { describe, expect, it } from 'vitest';
import { labelPortionMacros, validLabelPortion } from './product-label.ts';
import type { LabelEstimate } from './types.ts';

const label: LabelEstimate = {
 name: 'Oats', ingredients: 'Oats', basisQuantity: 100, basisUnit: 'g',
 fromNutritionLabel: true, confidence: 'high', notes: '',
 macros: { calories: 380, protein: 13, carbs: 60, fat: 7, fiber: 10 }
};

describe('product label portions', () => {
 it('scales every nutrient from a per-100-g label to the amount eaten', () => {
  expect(labelPortionMacros(label, 50)).toEqual({ calories: 190, protein: 6.5, carbs: 30, fat: 3.5, fiber: 5 });
 });
 it('handles liquids and serving-only panels without assuming a serving weight', () => {
  expect(labelPortionMacros({ ...label, basisUnit: 'ml' }, 250).calories).toBe(950);
  expect(labelPortionMacros({ ...label, basisQuantity: 1, basisUnit: 'serving' }, 0.5).calories).toBe(190);
 });
 it('uses corrected label values immediately', () => {
  expect(labelPortionMacros({ ...label, macros: { ...label.macros, calories: 400 } }, 40).calories).toBe(160);
 });
 it('blocks missing amounts and invalid label values', () => {
  for (const amount of [0, -1, NaN, Infinity]) expect(validLabelPortion(label, amount)).toBe(false);
  expect(validLabelPortion({ ...label, basisQuantity: 0 }, 50)).toBe(false);
  expect(validLabelPortion({ ...label, macros: { ...label.macros, protein: -1 } }, 50)).toBe(false);
  expect(validLabelPortion(null, 50)).toBe(false);
 });
});
