import { describe, expect, it } from 'vitest';
import { mergeProductSuggestion } from './product-ai.ts';
import type { Scan } from './types.ts';
const suggested: Scan = {name:'Cheddar',ingredients:'Milk, salt',hasNutrition:true,basisQuantity:100,basisUnit:'g',source:'ai',notes:'AI estimate',macros:{calories:410,protein:25,carbs:0,fat:35,saturatedFat:27,fiber:0}};
describe('reviewed AI suggestions', () => {
 it('preserves label values and scales only missing saturated fat to their basis', () => {
  const base: Scan = {...suggested,source:'saved',productId:'keep-id',ingredients:'Known ingredients',basisQuantity:25,macros:{calories:100,protein:6,carbs:1,fat:8,fiber:0,saturatedFat:null}};
  const merged=mergeProductSuggestion(base,suggested);
  expect(merged.productId).toBe('keep-id');
  expect(merged.ingredients).toBe('Known ingredients');
  expect(merged.macros).toEqual({...base.macros,saturatedFat:6.75});
  expect(merged.source).toBe('ai');
  expect(base.macros.saturatedFat).toBeNull();
 });
 it('fills missing ingredients and nutrition without retaining a calculated score', () => {
  const base: Scan={...suggested,hasNutrition:false,ingredients:'',rating:{score:92,base:90,label:'excellent',factors:[]}};
  const merged=mergeProductSuggestion(base,suggested);
  expect(merged.ingredients).toBe(suggested.ingredients);expect(merged.macros).toEqual(suggested.macros);expect(merged.rating).toBeUndefined();
 });
 it('does not guess serving weights or overwrite complete nutrition', () => {
  expect(()=>mergeProductSuggestion({...suggested,basisUnit:'serving',macros:{...suggested.macros,saturatedFat:null}},suggested)).toThrow('no missing information');
  expect(()=>mergeProductSuggestion(suggested,suggested)).toThrow('no missing information');
 });
});
