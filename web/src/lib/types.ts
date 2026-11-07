// Mirrors backend/internal/models. Keep in sync when you change the Go structs.
export interface Macros {
	calories: number;
	protein: number;
	carbs: number;
	fat: number;
	fiber: number;
}
export type MacroKey = keyof Macros;
export type MealType = 'breakfast' | 'lunch' | 'dinner' | 'snack';
export const MEAL_TYPES: MealType[] = ['breakfast', 'lunch', 'dinner', 'snack'];

export interface Profile {
	name: string;
	sex: 'male' | 'female' | '';
	age: number;
	heightCm: number;
	weightKg: number;
	activity: string;
	goal: 'lose' | 'maintain' | 'gain';
	targets: Macros;
	diets: string[];
	excluded: string[];
	allergies: string[];
	cuisines: string[];
	mealsPerDay: number;
	householdSize: number;
	maxPrepMins: number;
}

export interface Workout {
	id: string;
	date: string;
	kind: string;
	minutes: number;
	calories?: number;
	note?: string;
}

export interface Scan {
	id?: string;
	name: string;
	barcode?: string;
	ingredients: string;
	hasNutrition: boolean;
	basisQuantity?: number;
	basisUnit?: 'g' | 'ml' | 'serving';
	macros: Macros;
	source: 'saved' | 'openfoodfacts' | 'photo';
	productId?: string;
	notes?: string;
	warning?: string;
	createdAt?: string;
	rating?: ProductRating;
}

export interface WeightEntry {
	date: string;
	kg: number;
}

export interface SavedMeal {
	id: string;
	name: string;
	mealType: 'breakfast' | 'lunch' | 'dinner' | 'snack';
	items: MealItem[];
	macros: Macros;
	createdAt: string;
	updatedAt: string;
}

export interface Ingredient {
	name: string;
	quantity: number;
	unit: string;
	category: string;
}

export interface Recipe {
	id: string;
	title: string;
	description: string;
	mealType: MealType;
	servings: number;
	prepMinutes: number;
	ingredients: Ingredient[];
	steps: string[];
	macros: Macros;
	tags: string[];
	warnings?: string[];
	source?: 'plan' | 'pantry';
	usesPantry?: string[];
	missing?: Ingredient[];
	createdAt: string;
	avgRating: number;
	ratingCount: number;
}

export interface Rating {
	id: string;
	recipeId: string;
	stars: number;
	comment: string;
	createdAt: string;
}

export interface MealPlan {
	id: string;
	title: string;
	notes: string;
	days: { day: number; recipeIds: string[] }[];
	createdAt: string;
	mealCount?: number;
}

export interface PlanDetail {
	plan: MealPlan;
	recipes: Record<string, Recipe>;
}

export interface ShoppingItem {
	key: string;
	name: string;
	quantity: number;
	unit: string;
	category: string;
	checked: boolean;
	manual: boolean;
}

export interface ShoppingList {
	planId: string;
	items: ShoppingItem[];
}

export interface MealItem {
	name: string;
	quantity: string;
}

export interface LabelEstimate extends Estimate {
	ingredients: string;
	basisQuantity: number;
	basisUnit: 'g' | 'ml' | 'serving';
	fromNutritionLabel: boolean;
	barcode?: string;
	source?: '' | 'openfoodfacts';
}

export interface RatingFactor {
	text: string;
	points: number;
	why?: string;
}

export interface ProductRating {
	rules?: number;
	at?: string;
	base: number;
	provisional?: boolean;
	score: number;
	label: 'excellent' | 'good' | 'fair' | 'poor' | 'avoid';
	factors: RatingFactor[];
}

export interface Product extends LabelEstimate {
	rating?: ProductRating;
	id: string;
	createdAt: string;
	updatedAt: string;
}

export interface PhotoEstimate extends Estimate {
	items: MealItem[];
}

export interface LogEntry {
	items?: MealItem[];
	id: string;
	date: string;
	mealType: MealType;
	name: string;
	servings: number;
	macros: Macros;
	recipeId?: string;
	createdAt: string;
}

export interface DayAdjustment {
	calories: number;
	note: string;
}

export interface DayLog {
	baseTargets: Macros;
	adjustment: DayAdjustment;
	date: string;
	entries: LogEntry[];
	totals: Macros;
	targets: Macros;
}

export interface History {
	days: { date: string; totals: Macros; targets: Macros }[];
	targets: Macros;
}

export interface Estimate {
	name: string;
	macros: Macros;
	confidence: string;
	notes: string;
}

export interface Feedback {
	loved: { title: string; avg: number; comments: string[] | null }[] | null;
	disliked: { title: string; avg: number; comments: string[] | null }[] | null;
	likedIngredients: string[] | null;
	dislikedIngredients: string[] | null;
	likedTags: string[] | null;
	ratingCount: number;
}

export interface LLMConfig {
	provider: 'mock' | 'openai' | 'anthropic';
	baseUrl: string;
	model: string;
	apiKey: string;
	temperature: number;
	maxTokens: number;
	timeoutSeconds: number;
	jsonMode: boolean;
	visionModel: string;
}

export interface Settings {
	llm: LLMConfig;
	lookup: { openFoodFacts: boolean };
	envKeyLoaded: boolean;
	canEdit: boolean;
}

export interface User {
	id: string;
	email: string;
	name: string;
	admin: boolean;
	local?: boolean; // running inside the Android app: one account, data on the phone
}

export interface PantryItem {
	id: string;
	name: string;
	quantity: string;
	category: string;
	source: 'manual' | 'photo';
	addedAt: string;
}

export interface DetectedItem {
	name: string;
	quantity: string;
	category: string;
	confidence: string;
	have: boolean;
}

export const PANTRY_CATEGORIES = ['produce', 'protein', 'dairy', 'grains', 'pantry', 'frozen', 'drinks', 'other'];

export const emptyMacros = (): Macros => ({ calories: 0, protein: 0, carbs: 0, fat: 0, fiber: 0 });
