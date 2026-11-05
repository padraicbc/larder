import { emptyMacros, type LabelEstimate, type Macros } from './types.ts';

export function validLabelPortion(label: LabelEstimate | null, amount: number): boolean {
	return !!label && Number.isFinite(amount) && amount > 0 &&
		Number.isFinite(label.basisQuantity) && label.basisQuantity > 0 &&
		Object.values(label.macros).every((value) => Number.isFinite(value) && value >= 0 && Number.isFinite(value * (amount / label.basisQuantity)));
}

export function labelPortionMacros(label: LabelEstimate | null, amount: number): Macros {
	if (!validLabelPortion(label, amount) || !label) return emptyMacros();
	const factor = amount / label.basisQuantity;
	return {
		calories: label.macros.calories * factor,
		protein: label.macros.protein * factor,
		carbs: label.macros.carbs * factor,
		fat: label.macros.fat * factor,
		fiber: label.macros.fiber * factor
	};
}
