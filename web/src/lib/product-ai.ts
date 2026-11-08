import type { Scan } from './types.ts';

/** Fill gaps only. Values already copied from a label keep their original basis. */
export function mergeProductSuggestion(base: Scan | null, suggestion: Scan): Scan {
 if (!base) return { ...suggestion, source: 'ai', rating: undefined };
 const next: Scan = { ...base, macros: { ...base.macros }, rating: undefined };
 let used = false;
 if (!base.ingredients.trim() && suggestion.ingredients.trim()) { next.ingredients = suggestion.ingredients; used = true; }
 if (!base.hasNutrition && suggestion.hasNutrition) {
  next.hasNutrition = true;
  next.macros = { ...suggestion.macros };
  next.basisQuantity = suggestion.basisQuantity;
  next.basisUnit = suggestion.basisUnit;
  used = true;
 } else if (base.hasNutrition && base.macros.saturatedFat == null && suggestion.hasNutrition && suggestion.macros.saturatedFat != null && base.basisUnit === suggestion.basisUnit && base.basisQuantity && suggestion.basisQuantity) {
  const value = suggestion.macros.saturatedFat * base.basisQuantity / suggestion.basisQuantity;
  if (value >= 0 && value <= base.macros.fat) { next.macros.saturatedFat = value; used = true; }
 }
 if (!used) throw new Error('The suggestion has no missing information to add. Your existing values have been kept.');
 next.source = 'ai';
 next.notes = [base.notes, suggestion.notes].filter(Boolean).join('\n');
 return next;
}
