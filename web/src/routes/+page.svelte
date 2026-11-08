<script lang="ts">
 import { startFoodChat } from '#lib/start-food-chat.ts';
	import { goto, invalidateAll } from '$app/navigation';
	import { page } from '$app/state';
	import { api, upload, errorText } from '#lib/api.ts';
	import { addDays, cap, friendlyDate, longDate, ratio, round, todayISO } from '#lib/format.ts';
	import RatingBadge from '#lib/components/RatingBadge.svelte';
	import BarcodeField from '#lib/components/BarcodeField.svelte';
	import { lookupBarcode } from '#lib/barcode.ts';
	import { labelPortionMacros, validLabelPortion } from '#lib/product-label.ts';
	import { shrinkImage } from '#lib/images.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { MEAL_TYPES, emptyMacros, type Estimate, type LabelEstimate, type EverydayFood, type Product, type SavedMeal, type ProductRating, type PhotoEstimate, type MealItem, type LogEntry, type MacroKey, type Macros, type MealType } from '#lib/types.ts';
	import MacroRings from '#lib/components/MacroRings.svelte';
	import MacroBars from '#lib/components/MacroBars.svelte';
	import MacroLine from '#lib/components/MacroLine.svelte';
	import MacroStripe from '#lib/components/MacroStripe.svelte';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	const today = todayISO();
	const isToday = $derived(data.date === today);
	const burned = $derived(data.workouts.reduce((n, w) => n + (w.calories ?? 0), 0));
	const workoutMins = $derived(data.workouts.reduce((n, w) => n + w.minutes, 0));
	const byMeal = $derived(
		MEAL_TYPES.map((t) => [t, data.day.entries.filter((e) => e.mealType === t)] as [MealType, LogEntry[]])
	);

	const go = (date: string) => goto(date === today ? '/' : `/?date=${date}`);

	async function remove(e: LogEntry) {
		try {
			await api(`/log/${e.id}`, { method: 'DELETE' });
			await invalidateAll();
			toast(`Removed ${e.name}`, 'info');
		} catch (err) {
			toast(errorText(err), 'error');
		}
	}

	let targetDialog: HTMLDialogElement;
	let dayCalories = $state(0);
	let targetNote = $state('');
	let savingTarget = $state(false);
	const calorieChange = $derived(dayCalories - data.day.baseTargets.calories);
	const validTarget = $derived(Number.isFinite(dayCalories) && dayCalories > 0);

	function openTarget() {
		dayCalories = data.day.targets.calories;
		targetNote = data.day.adjustment.note;
		targetDialog.showModal();
	}

	async function saveTarget(reset = false) {
		if (savingTarget || (!reset && !validTarget)) return;
		savingTarget = true;
		try {
			await api(`/log/target?date=${data.date}`, { method: 'PUT', body: {
				calories: reset ? 0 : calorieChange, note: reset ? '' : targetNote
			} });
			await invalidateAll();
			targetDialog.close();
			toast(reset ? 'Usual calorie target restored for this day' : 'Calorie target updated for this day');
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			savingTarget = false;
		}
	}

	// ---------- add-food dialog ----------
	let dialog: HTMLDialogElement;
	type Mode = 'products' | 'recent' | 'describe' | 'photo' | 'recipe' | 'manual';
	let mode = $state<Mode>('describe');
	let mealType = $state<MealType>('breakfast');
	let text = $state('');
	let estimate = $state<Estimate | null>(null);
	let name = $state('');
	let servings = $state(1);
	let macros = $state(emptyMacros());
	let recipeId = $state('');
	let busy = $state(false);
	let mealItems = $state<MealItem[]>([]);
	let estimatedItems = $state('');
	let photoInput = $state<HTMLInputElement>();
	let photoKind = $state<'meal' | 'label'>('meal');
	let labelResult = $state<LabelEstimate | null>(null);
	let labelAmount = $state(0);
	let savedProductId = $state('');
	let savedProductSnapshot = $state('');
	let savedRating = $state<ProductRating | null>(null);
	const RATING_CLASS = { excellent: 'badge-success', good: 'badge-success badge-soft', fair: 'badge-warning', poor: 'badge-error badge-soft', avoid: 'badge-error' } as const;
	let productSearch = $state('');
	let savingProduct = $state(false);
	const productSnapshot = $derived(labelResult ? JSON.stringify({ ...labelResult, name: name.trim() }) : '');
	const productChanged = $derived(productSnapshot !== savedProductSnapshot);
	const shownRating = $derived(savedProductId && !productChanged ? savedRating : null);
	const productValid = $derived(!!name.trim() && !!labelResult && validLabelPortion(labelResult, labelResult.basisQuantity));
	const matchingProducts = $derived(data.products.filter((product) => (product.name + ' ' + (product.barcode ?? '')).toLowerCase().includes(productSearch.toLowerCase())));

	function useProduct(product: Product) {
		labelResult = { ...product, macros: { ...product.macros } };
		estimate = labelResult;
		name = product.name;
		labelAmount = 0;
		servings = 1;
		savedProductId = product.id;
		savedRating = product.rating ?? null;
		savedProductSnapshot = JSON.stringify({ ...labelResult, name: name.trim() });
	}

	// ---- everyday foods: log eggs, chicken, potatoes… without setting anything up first ----
	let foods = $state<EverydayFood[]>([]);
	let foodsLoaded = false;
	let pickingFood = $state(false);
	async function ensureFoods() {
		if (foodsLoaded) return;
		foodsLoaded = true;
		try {
			foods = (await api<{ foods: EverydayFood[] }>('/foods')).foods;
		} catch {
			foodsLoaded = false; // try again next time the tab opens
		}
	}
	const QUICK_FOODS = ['Eggs (boiled)', 'Eggs (scrambled)', 'Chicken breast (cooked)', 'Bacon (cooked)', 'Bread (white)', 'Toast (wholemeal)', 'Milk (semi-skimmed)', 'Rice (white, cooked)', 'Pasta (cooked)', 'Porridge oats (dry)', 'Bananas', 'Apples', 'Potatoes (boiled)'];
	// Browse by kind of food: tap a category to list what's in it.
	const CATEGORY_ORDER = ['Eggs', 'Meat & poultry', 'Fish & seafood', 'Dairy', 'Bread, rice & pasta', 'Vegetables', 'Fruit', 'Beans & pulses', 'Nuts & seeds', 'Fats & oils', 'Sweet & spreads', 'Drinks'];
	let foodCategory = $state('');
	const categories = $derived(CATEGORY_ORDER.filter((c) => foods.some((f) => f.category === c)));
	const categoryFoods = $derived(foods.filter((f) => f.category === foodCategory).toSorted((a, b) => a.name.localeCompare(b.name)));
	const quickFoods = $derived(QUICK_FOODS.map((n) => foods.find((f) => f.name === n)).filter((f): f is EverydayFood => !!f));
	const matchingFoods = $derived.by(() => {
		const q = productSearch.trim().toLowerCase();
		if (!q) return [];
		const rank = (f: EverydayFood) => (f.name.toLowerCase().startsWith(q) ? 0 : f.name.toLowerCase().includes(q) ? 1 : f.aliases.some((a) => a.toLowerCase().includes(q)) ? 2 : -1);
		return foods.filter((f) => rank(f) >= 0).toSorted((a, b) => rank(a) - rank(b)).slice(0, 8);
	});

	/** Pick an everyday food: reuse it if it's already one of your products, otherwise save it (generic, USDA values) and select it. */
	async function useEveryday(f: EverydayFood) {
		const have = data.products.find((p) => p.name.toLowerCase() === f.name.toLowerCase());
		if (have) {
			useProduct(have);
			return;
		}
		pickingFood = true;
		try {
			const saved = await api<Product>('/products', {
				body: {
					name: f.name,
					generic: true,
					source: 'usda',
					fromNutritionLabel: true,
					confidence: 'medium',
					basisQuantity: 100,
					basisUnit: 'g',
					macros: { calories: f.calories, protein: f.protein, carbs: f.carbs, fat: f.fat, fiber: f.fiber },
					portions: f.portions,
					notes: `Per 100 g. From USDA FoodData Central (SR Legacy #${f.fdcId}): ${f.usda}.`
				}
			});
			await invalidateAll(); // so it's in your products from now on
			useProduct(saved);
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			pickingFood = false;
		}
	}

	let barcodeBusy = $state(false);
	let barcodeMsg = $state('');
	async function findByBarcode(code: string) {
		barcodeBusy = true;
		barcodeMsg = '';
		try {
			const found = await lookupBarcode(code);
			if (found.source === 'saved') {
				useProduct(found.product);
				return;
			}
			const result = found.product;
			labelResult = result;
			labelAmount = 0;
			estimate = result;
			name = result.name;
			servings = 1;
			savedProductId = '';
			savedProductSnapshot = '';
			savedRating = null;
			await persistProduct(); // keep it in Products so it's found offline next time
			toast('Found on Open Food Facts and saved to Products. Check the values, then enter how much you ate.', 'info');
		} catch (err) {
			barcodeMsg = errorText(err);
		} finally {
			barcodeBusy = false;
		}
	}

	async function persistProduct() {
		if (!labelResult || !productValid) throw new Error('Check the product name and nutrition values first.');
		if (savedProductId && !productChanged) return;
		const saved = await api<Product>('/products', { body: { ...labelResult, id: savedProductId, name: name.trim() } });
		savedProductId = saved.id;
		savedRating = saved.rating ?? null;
		savedProductSnapshot = JSON.stringify({ ...labelResult, name: name.trim() });
	}

	async function saveProductForLater() {
		if (savingProduct || !productValid) return;
		savingProduct = true;
		try {
			await persistProduct();
			await invalidateAll();
			toast('Product saved. Find it in Products next time.');
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			savingProduct = false;
		}
	}

	const labelMacros = $derived(labelPortionMacros(labelResult, labelAmount));
	const labelValid = $derived(validLabelPortion(labelResult, labelAmount));

	function choosePhotoKind(kind: 'meal' | 'label') {
		photoKind = kind;
		savedProductId = '';
		savedProductSnapshot = '';
		labelResult = null;
		estimate = null;
		mealItems = [];
		name = '';
		servings = 1;
	}
	const itemsValid = $derived(mealItems.length > 0 && mealItems.every((i) => i.name.trim() && i.quantity.trim()));
	const photoDirty = $derived(JSON.stringify(mealItems) !== estimatedItems);

	async function onMealPhoto(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		busy = true;
		estimate = null;
		labelResult = null;
		savedProductId = '';
		savedProductSnapshot = '';
		mealItems = [];
		try {
			const form = new FormData();
			form.append('photos', await shrinkImage(file), 'meal.jpg');
			if (photoKind === 'label') {
				const result = await upload<LabelEstimate>('/log/label', form);
				labelResult = result;
				labelAmount = 0;
				estimate = result;
				name = result.name;
				servings = 1;
				const known = result.barcode && data.products.find((p) => p.barcode === result.barcode);
				if (known) toast(`This barcode is already saved as ${known.name}. Saving will update it.`, 'info');
			} else {
				const result = await upload<PhotoEstimate>('/log/photo', form);
				mealItems = result.items;
				estimatedItems = JSON.stringify(mealItems);
				estimate = result;
				name = result.name;
				macros = { ...result.macros };
			}
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			busy = false;
		}
	}

	async function updatePhotoEstimate() {
		busy = true;
		const snapshot = JSON.stringify(mealItems);
		try {
			const result = await api<Estimate>('/log/estimate', {
				body: { text: 'One serving containing exactly these foods and quantities: ' + mealItems.map((i) => `${i.quantity} ${i.name}`).join('; ') }
			});
			estimate = result;
			macros = { ...result.macros };
			estimatedItems = snapshot;
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			busy = false;
		}
	}

	const sortedRecipes = $derived([...data.recipes].sort((a, b) => a.title.localeCompare(b.title)));
	const chosen = $derived(data.recipes.find((r) => r.id === recipeId));
	const scaled = (m: Macros, f: number): Macros => ({
		calories: m.calories * f,
		protein: m.protein * f,
		carbs: m.carbs * f,
		fat: m.fat * f,
		fiber: m.fiber * f
	});

	function guessMeal(): MealType {
		const h = new Date().getHours() + new Date().getMinutes() / 60;
		return h < 10.5 ? 'breakfast' : h < 15 ? 'lunch' : h < 21 ? 'dinner' : 'snack';
	}

	let repeating = $state(false);
	let recentSearch = $state('');
	const matchingRecent = $derived(data.recentLogs.filter((e) => e.name.toLowerCase().includes(recentSearch.toLowerCase())));

	async function repeatMeal(entry: LogEntry, targetMeal: MealType = entry.mealType) {
		if (repeating || busy) return;
		repeating = true;
		try {
			await api(`/log/${entry.id}/repeat`, { body: { date: data.date, mealType: targetMeal } });
			if (dialog?.open) dialog.close();
			await invalidateAll();
			toast(`Logged ${entry.name}`);
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			repeating = false;
		}
	}

	const matchingMeals = $derived(data.meals.filter((m) => m.name.toLowerCase().includes(recentSearch.toLowerCase())));
	async function logSavedMeal(meal: SavedMeal) {
		if (repeating || busy) return;
		repeating = true;
		try {
			await api(`/meals/${meal.id}/log`, { body: { date: data.date, mealType, servings } });
			if (dialog?.open) dialog.close();
			await invalidateAll();
			toast(`Logged ${meal.name}`);
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			repeating = false;
		}
	}

	// ---- group several logged foods into one named, reusable meal ----
	let groupType = $state<MealType | null>(null);
	let picked = $state<string[]>([]);
	let groupName = $state('');
	let groupMerge = $state(true);
	let grouping = $state(false);
	function startGroup(type: MealType) {
		groupType = type;
		picked = [];
		groupName = '';
		groupMerge = true;
	}
	async function saveGroup() {
		if (!groupType || !picked.length || !groupName.trim() || grouping) return;
		grouping = true;
		try {
			await api('/meals/group', { body: { name: groupName.trim(), mealType: groupType, entryIds: picked, merge: groupMerge } });
			toast(`Saved “${groupName.trim()}”. Find it under Log food → Recent.`);
			groupType = null;
			await invalidateAll();
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			grouping = false;
		}
	}

	function adjustRecent(entry: LogEntry) {
		mode = 'manual';
		name = entry.name;
		servings = entry.servings > 0 ? entry.servings : 1;
		macros = scaled(entry.macros, 1 / servings);
		mealItems = (entry.items ?? []).map((item) => ({ ...item }));
		estimate = null;
	}

	function openAdd(type?: MealType) {
		mealType = type ?? guessMeal();
		mode = data.recentLogs.length ? 'recent' : 'describe';
		recentSearch = '';
		text = name = recipeId = '';
		estimate = null;
		mealItems = [];
		estimatedItems = '';
		labelResult = null;
		photoKind = 'meal';
		savedProductId = '';
		savedProductSnapshot = '';
		productSearch = '';
		servings = 1;
		macros = emptyMacros();
		dialog.showModal();
	}

	async function runEstimate() {
		busy = true;
		try {
			const e = await api<Estimate>('/log/estimate', { body: { text } });
			estimate = e;
			name = e.name;
			macros = { ...e.macros };
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			busy = false;
		}
	}

	const canSave = $derived(mode === 'recipe' ? !!recipeId : name.trim() !== '' && (mode === 'manual' || (estimate !== null && (mode === 'products' ? labelValid : mode !== 'photo' || (labelResult ? labelValid : itemsValid && !photoDirty)))));

	async function save() {
		busy = true;
		try {
			if ((mode === 'photo' || mode === 'products') && labelResult) await persistProduct();
			const body =
				mode === 'recipe'
					? { date: data.date, mealType, servings, recipeId }
					: { date: data.date, mealType, servings, name, macros: (mode === 'photo' || mode === 'products') && labelResult ? labelMacros : macros, ...((mode === 'photo' || mode === 'products') && labelResult ? { items: [{ name, quantity: `${labelAmount} ${labelResult.basisUnit}` }] } : (mode === 'photo' || mode === 'manual') && mealItems.length ? { items: mealItems } : {}) };
			const e = await api<LogEntry>('/log', { body });
			dialog.close();
			await invalidateAll();
			toast(`Logged ${e.name}`);
		} catch (err) {
			toast(errorText(err), 'error');
		} finally {
			busy = false;
		}
	}

	// Home-screen shortcut "Log food" opens /?add=1
	$effect(() => {
		if (page.url.searchParams.get('add') === '1') {
			openAdd();
			goto('/', { replaceState: true });
		}
	});

	const FIELDS: { key: MacroKey; label: string; unit: string }[] = [
		{ key: 'calories', label: 'Calories', unit: 'kcal' },
		{ key: 'protein', label: 'Protein', unit: 'g' },
		{ key: 'carbs', label: 'Carbs', unit: 'g' },
		{ key: 'fat', label: 'Fat', unit: 'g' },
		{ key: 'fiber', label: 'Fiber', unit: 'g' }
	];
</script>

<svelte:head><title>{friendlyDate(data.date)} · Larder</title></svelte:head>

<header class="flex items-end justify-between gap-3">
	<div>
		{#if friendlyDate(data.date) !== longDate(data.date)}
			<p class="text-base-content/60 text-sm font-semibold">{longDate(data.date)}</p>
		{/if}
		<h1 class="text-4xl font-extrabold lg:text-5xl">{friendlyDate(data.date)}</h1>
	</div>
	<div class="flex items-center gap-2">
		{#if !isToday}
			<button class="btn btn-ghost btn-sm" onclick={() => go(today)}>Today</button>
		{/if}
		<div class="join">
			<button class="btn btn-sm join-item" aria-label="Previous day" onclick={() => go(addDays(data.date, -1))}>
				<Icon name="left" size={18} />
			</button>
			<button class="btn btn-sm join-item" aria-label="Next day" disabled={isToday} onclick={() => go(addDays(data.date, 1))}>
				<Icon name="right" size={18} />
			</button>
		</div>
		<button class="btn btn-primary max-lg:hidden" onclick={() => openAdd()}><Icon name="plus" />Log food</button>
	</div>
</header>

<a class="btn btn-ghost btn-sm mt-2" href="/chat"><Icon name="sparkle" size={16} />Food chat · ask, look up and plan</a>

<!-- Phones: Log food floats above the nav bar, within thumb reach. -->
<button
	class="btn btn-primary btn-lg fixed right-4 z-20 rounded-full shadow-lg lg:hidden"
	style="bottom: calc(5.25rem + env(safe-area-inset-bottom))"
	onclick={() => openAdd()}
>
	<Icon name="plus" />Log food
</button>

<div class="mt-3 flex gap-2 lg:hidden">
	<a class="btn btn-sm btn-soft" href="/scan">Shop check</a>
	<a class="btn btn-sm btn-soft" href="/workouts">Workouts</a>
	<a class="btn btn-sm btn-soft" href="/weight">Weight</a>
	<a class="btn btn-sm btn-soft" href="/meals">Meals</a>
</div>

<section class="mt-6 grid items-center gap-6 sm:grid-cols-[auto_1fr] sm:gap-10">
	<div class="mx-auto w-56 sm:w-64">
		<MacroRings totals={data.day.totals} targets={data.day.targets} />
	</div>
	<div>
		{#if data.day.entries.length}
			<p class="tabular mb-4 text-lg">
				<span class="font-display text-2xl font-bold">{round(data.day.totals.calories)}</span>
				<span class="text-base-content/60">of {round(data.day.targets.calories)} kcal eaten</span>
			</p>
		{:else}
			<p class="mb-4 text-lg">
				Nothing logged {isToday ? 'yet today' : 'this day'}.
				<span class="text-base-content/60">Describe a meal in your own words and the macros are worked out for you.</span>
			</p>
		{/if}
		{#if data.workouts.length}
			<p class="tabular -mt-2 mb-4 text-sm">
				<a class="link-hover" href="/workouts">
					<span class="font-semibold">{burned ? `${round(burned)} kcal burned` : `${workoutMins} min trained`}</span>
					<span class="text-base-content/60">
						· {data.workouts.map((w) => w.kind).join(', ')}{burned ? ` · ${workoutMins} min` : ''}
					</span>
				</a>
			</p>
		{/if}
		<MacroBars totals={data.day.totals} targets={data.day.targets} />
		<button class="btn btn-ghost btn-sm mt-3 -ml-3" onclick={openTarget}><Icon name="settings" size={16} />Adjust this day’s calories</button>
		{#if data.day.adjustment.calories || data.day.adjustment.note}
			<p class="text-base-content/60 mt-1 text-sm">{data.day.adjustment.calories >= 0 ? '+' : ''}{round(data.day.adjustment.calories)} kcal for this day{data.day.adjustment.note ? ` · ${data.day.adjustment.note}` : ''}</p>
		{/if}
	</div>
</section>

{#if data.recentLogs.length}
	<section class="mt-6" aria-label="Quick repeat meals">
		<h2 class="mb-2 text-sm font-semibold text-base-content/60">Log again</h2>
		<div class="flex gap-2 overflow-x-auto pb-2">
			{#each data.recentLogs.slice(0, 6) as entry (entry.id)}
				<button class="btn btn-soft shrink-0" disabled={repeating || busy} onclick={() => repeatMeal(entry)} aria-label="Log {entry.name} again for {friendlyDate(data.date)}">
					<Icon name="plus" size={16} />{entry.name}<span class="text-xs font-normal opacity-60">{round(entry.macros.calories)} kcal</span>
				</button>
			{/each}
		</div>
	</section>
{/if}

<!-- Week strip: tap a day to open it -->
<section class="mt-8" aria-label="Calories this week">
	<ol class="grid grid-cols-7 gap-2">
		{#each data.history.days as d (d.date)}
			{@const frac = ratio(d.totals.calories, d.targets.calories)}
			{@const over = d.totals.calories > d.targets.calories * 1.05}
			<li>
				<button
					class="flex w-full flex-col items-center gap-1 rounded-box p-1.5 transition-colors {d.date === data.date
						? 'bg-base-300'
						: 'hover:bg-base-200'}"
					disabled={d.date > today}
					onclick={() => go(d.date)}
					aria-label="{friendlyDate(d.date)}: {round(d.totals.calories)} kcal"
				>
					<span class="bg-base-200 relative h-16 w-3 overflow-hidden rounded-full">
						<span
							class="absolute inset-x-0 bottom-0 rounded-full {over ? 'bg-warning' : 'bg-primary'}"
							style="height: {frac * 100}%"
						></span>
					</span>
					<span class="text-xs font-semibold text-base-content/70">
						{new Date(d.date + 'T12:00').toLocaleDateString(undefined, { weekday: 'narrow' })}
					</span>
				</button>
			</li>
		{/each}
	</ol>
</section>

<section class="mt-10 flex flex-col gap-8 pb-16 lg:pb-0">
	{#each byMeal as [type, entries] (type)}
		{@const kcal = entries.reduce((n, e) => n + e.macros.calories, 0)}
		<div>
			<div class="border-base-content/15 flex items-baseline justify-between border-b pb-1.5">
				<h2 class="text-xl font-bold">{cap(type)}</h2>
				<div class="flex items-center gap-1">
					{#if kcal}<span class="tabular text-base-content/60 text-sm">{round(kcal)} kcal</span>{/if}
					{#if entries.length > 1 && groupType !== type}
						<button class="btn btn-ghost btn-xs" onclick={() => startGroup(type)}>Group as meal</button>
					{/if}
					<button class="btn btn-ghost btn-square btn-sm" onclick={() => openAdd(type)} aria-label="Add to {type}">
						<Icon name="plus" size={18} />
					</button>
				</div>
			</div>
			{#if entries.length === 0}
				<p class="text-base-content/45 py-3 text-sm">Nothing yet</p>
			{:else}
				<ul class="divide-base-content/10 divide-y">
					{#each entries as e (e.id)}
						<li class="flex items-center gap-4 py-3">
							{#if groupType === type}
								<input type="checkbox" class="checkbox checkbox-sm" aria-label="Include {e.name}" bind:group={picked} value={e.id} />
							{/if}
							<div class="min-w-0 flex-1">
								<p class="truncate font-semibold">
									{#if e.recipeId}<a class="link-hover" href="/recipes/{e.recipeId}">{e.name}</a>{:else}{e.name}{/if}
									{#if e.servings !== 1}<span class="text-base-content/50 font-normal"> × {e.servings}</span>{/if}
								</p>
								<!-- Quantities stay visible after a photo meal is logged. -->
								{#if e.items?.length}<p class="text-base-content/60 mt-1 text-xs">{e.items.map((i) => `${i.quantity} ${i.name}`).join(', ')}</p>{/if}
								<div class="mt-1.5 max-w-56"><MacroStripe m={e.macros} height={5} /></div>
							</div>
							<div class="text-right">
								<p class="font-display tabular text-lg font-bold leading-none">{round(e.macros.calories)}</p>
								<p class="text-base-content/50 text-xs">kcal</p>
							</div>
							<button class="btn btn-ghost btn-square btn-sm -mr-2" aria-label="Remove {e.name}" onclick={() => remove(e)}>
								<Icon name="trash" size={16} />
							</button>
						</li>
					{/each}
				</ul>
				{#if groupType === type}
					<div class="bg-base-200 rounded-box mt-2 flex flex-col gap-2 p-3">
						<p class="text-sm">Tick the foods that make one meal, then name it. You can log it again in one tap.</p>
						<input class="input w-full" placeholder="e.g. Usual breakfast" aria-label="Meal name" bind:value={groupName} />
						<label class="flex cursor-pointer items-center gap-2 text-sm">
							<input type="checkbox" class="checkbox checkbox-sm" bind:checked={groupMerge} />Combine them into one entry in today’s log
						</label>
						<div class="flex gap-2">
							<button class="btn btn-primary btn-sm" disabled={!picked.length || !groupName.trim() || grouping} onclick={saveGroup}>Save meal ({picked.length})</button>
							<button class="btn btn-ghost btn-sm" onclick={() => (groupType = null)}>Cancel</button>
						</div>
					</div>
				{/if}
			{/if}
		</div>
	{/each}
</section>

<dialog bind:this={dialog} class="modal modal-bottom sm:modal-middle" oncancel={(event) => { if (busy || repeating || savingProduct) event.preventDefault(); }}>
	<div class="modal-box">
		<fieldset disabled={busy || repeating || savingProduct}>
		<h3 class="text-2xl font-bold">Log food</h3>
 <button class="btn btn-ghost btn-sm mt-2" onclick={() => startFoodChat({kind: labelResult ? 'product' : 'meal', context: JSON.stringify(labelResult || {name, description: text, macros: estimate?.macros, items: mealItems})})}>Chat about this before logging</button>

		<div role="tablist" class="tabs tabs-box mt-4 grid grid-cols-3">
			<button role="tab" class="tab" class:tab-active={mode === 'recent'} onclick={() => mode = 'recent'}>Recent</button>
			<button role="tab" class="tab" class:tab-active={mode === 'describe'} onclick={() => { mode = 'describe'; mealItems = []; estimate = null; }}>
				Describe
			</button>
			<button role="tab" class="tab" class:tab-active={mode === 'photo'} onclick={() => { mode = 'photo'; choosePhotoKind('meal'); }}>Photo</button>
			<button role="tab" class="tab" class:tab-active={mode === 'products'} onclick={() => { mode = 'products'; labelResult = null; estimate = null; ensureFoods(); }}>Products</button>
			<button role="tab" class="tab" class:tab-active={mode === 'recipe'} onclick={() => (mode = 'recipe')}>
				Recipe
			</button>
			<button role="tab" class="tab" class:tab-active={mode === 'manual'} onclick={() => { if ((mode === 'photo' || mode === 'products') && labelResult && labelValid) { macros = { ...labelMacros }; mealItems = [{ name, quantity: `${labelAmount} ${labelResult.basisUnit}` }]; } mode = 'manual'; }}>
				Numbers
			</button>
		</div>

		<div class="mt-4 grid grid-cols-2 gap-3">
			<label class="floating-label">
				<span>Meal</span>
				<select class="select w-full" bind:value={mealType}>
					{#each MEAL_TYPES as t (t)}<option value={t}>{cap(t)}</option>{/each}
				</select>
			</label>
			<label class="floating-label">
				<span>Servings</span>
				<input class="input w-full" type="number" min="0.25" step="0.25" bind:value={servings} />
			</label>
		</div>

		{#if mode === 'recent'}
			<p class="text-base-content/60 mt-4 text-sm">Repeat a meal for {friendlyDate(data.date).toLowerCase()}. Your usual meals appear first.</p>
			<input class="input mt-3 w-full" type="search" aria-label="Find a previous meal" placeholder="Find porridge, coffee…" bind:value={recentSearch} />
			{#if matchingMeals.length}
				<p class="mt-3 text-sm font-semibold">Your saved meals</p>
				<ul class="divide-base-content/10 mt-1 max-h-48 divide-y overflow-y-auto">
					{#each matchingMeals as meal (meal.id)}
						<li class="flex items-center gap-2 py-3">
							<div class="min-w-0 flex-1">
								<p class="font-semibold">{meal.name}</p>
								<p class="text-base-content/60 text-xs">{round(meal.macros.calories)} kcal · {meal.items.map((i) => `${i.quantity} ${i.name}`).join(', ')}</p>
							</div>
							<button class="btn btn-primary btn-sm" onclick={() => logSavedMeal(meal)} aria-label="Log {meal.name}">Log</button>
						</li>
					{/each}
				</ul>
				<p class="mt-3 text-sm font-semibold">Recent</p>
			{/if}
			<ul class="mt-3 max-h-72 divide-y divide-base-content/10 overflow-y-auto">
				{#each matchingRecent as entry (entry.id)}
					<li class="flex items-center gap-2 py-3">
						<div class="min-w-0 flex-1"><p class="font-semibold">{entry.name}</p><p class="text-base-content/60 text-xs">{round(entry.macros.calories)} kcal · {entry.servings} {entry.servings === 1 ? 'serving' : 'servings'}</p></div>
						<button class="btn btn-ghost btn-sm" onclick={() => adjustRecent(entry)} aria-label="Adjust {entry.name}">Adjust</button>
						<button class="btn btn-primary btn-sm" onclick={() => repeatMeal(entry, mealType)} aria-label="Log {entry.name} again">Log again</button>
					</li>
				{:else}
					<li class="text-base-content/60 py-4 text-sm">{data.recentLogs.length ? 'No matching meals.' : 'Meals you log will appear here for next time.'}</li>
				{/each}
			</ul>
		{/if}

		{#if mode === 'describe'}
			<label class="mt-4 block">
				<span class="mb-1 block text-sm font-semibold">What did you eat?</span>
				<textarea
					class="textarea w-full"
					rows="2"
					placeholder="Two eggs on sourdough with half an avocado"
					bind:value={text}
				></textarea>
			</label>
			<button class="btn btn-secondary btn-soft mt-2 w-full" disabled={busy || !text.trim()} onclick={runEstimate}>
				{#if busy}<span class="loading loading-spinner loading-sm"></span>{:else}<Icon name="sparkle" size={18} />{/if}
				Estimate macros
			</button>
			{#if estimate}
				<p class="text-base-content/70 mt-3 text-sm">
					<span class="badge badge-sm badge-ghost">{estimate.confidence || 'unknown'} confidence</span>
					{estimate.notes}
				</p>
			{/if}
		{/if}


		{#if mode === 'photo'}
			<div class="join mt-4 w-full">
				<button class="btn join-item flex-1" class:btn-active={photoKind === 'meal'} onclick={() => choosePhotoKind('meal')}>Meal photo</button>
				<button class="btn join-item flex-1" class:btn-active={photoKind === 'label'} onclick={() => choosePhotoKind('label')}>Product label</button>
			</div>
			<input bind:this={photoInput} type="file" accept="image/*" class="hidden" onchange={onMealPhoto} />
			<button class="btn btn-secondary btn-soft mt-4 w-full" onclick={() => photoInput?.click()}>
				{#if busy}<span class="loading loading-spinner loading-sm"></span>{:else}<Icon name="camera" size={18} />{/if}
				{busy ? 'Reading your photo…' : estimate ? 'Use another photo' : photoKind === 'label' ? 'Take or choose a label photo' : 'Take or choose a meal photo'}
			</button>
			<p class="text-base-content/60 mt-2 text-xs">{photoKind === 'label' ? 'Photograph the nutrition panel clearly, including its per-100-g, per-100-ml or per-serving heading. You’ll enter how much you ate before logging.' : 'Check the foods and portions before logging. Photo estimates can miss ingredients, oils and sauces.'}</p>
			{#if !labelResult && (estimate || mealItems.length)}
				<div class="mt-4 space-y-2">
					{#each mealItems as item, i}
						<div class="flex items-center gap-2">
							<input class="input min-w-0 flex-1" aria-label="Food {i + 1}" placeholder="Food" bind:value={item.name} />
							<input class="input w-28" aria-label="Quantity {i + 1}" placeholder="e.g. 150 g" bind:value={item.quantity} />
							<button class="btn btn-ghost btn-square btn-sm" aria-label="Remove food {i + 1}" onclick={() => mealItems.splice(i, 1)}><Icon name="trash" size={16} /></button>
						</div>
					{/each}
				</div>
				<button class="btn btn-ghost btn-sm mt-2" onclick={() => mealItems.push({ name: '', quantity: '' })}><Icon name="plus" size={16} />Add missed food</button>
				{#if photoDirty}
					<p class="text-warning mt-2 text-sm">Update the estimate to include your changes before logging.</p>
					<button class="btn btn-secondary mt-2 w-full" disabled={!itemsValid || busy} onclick={updatePhotoEstimate}>Update estimate</button>
				{/if}
				{#if estimate}<p class="text-base-content/60 mt-3 text-sm">{estimate.confidence || 'unknown'} confidence. {estimate.notes}</p>{/if}
			{/if}
		{/if}

		{#if mode === 'products'}
			{#if labelResult}
				<!-- A food is chosen: keep the amount and quick buttons in view instead of the picker above them. -->
				<div class="bg-base-200 rounded-box mt-4 flex items-center justify-between gap-2 p-3">
					<span class="min-w-0 truncate font-semibold">{labelResult.name}</span>
					<button type="button" class="btn btn-ghost btn-sm" onclick={() => { labelResult = null; estimate = null; savedProductId = ''; }}>Change food</button>
				</div>
			{:else}
			<p class="text-base-content/65 mt-4 text-sm">Pick a food and say how much you ate. Calories and macros are worked out for you.</p>
			{#if !productSearch.trim() && quickFoods.length}
				<p class="mt-3 text-sm font-semibold">Everyday foods</p>
				<div class="mt-1 flex flex-wrap gap-1" aria-label="Everyday foods">
					{#each quickFoods as f (f.name)}<button type="button" class="btn btn-xs btn-outline border-base-300 rounded-full" disabled={pickingFood} onclick={() => useEveryday(f)}>{f.name}</button>{/each}
				</div>
				<p class="text-base-content/60 mt-3 text-xs">Or browse by kind of food:</p>
				<div class="mt-1 flex flex-wrap gap-1" aria-label="Food categories">
					{#each categories as c (c)}
						<button type="button" class="btn btn-xs rounded-full {foodCategory === c ? 'btn-primary' : 'btn-ghost bg-base-200'}" aria-pressed={foodCategory === c} onclick={() => (foodCategory = foodCategory === c ? '' : c)}>{c}</button>
					{/each}
				</div>
				{#if foodCategory}
					<ul class="divide-base-content/10 mt-2 max-h-56 divide-y overflow-y-auto" aria-label="Foods in {foodCategory}">
						{#each categoryFoods as f (f.name)}
							<li><button type="button" class="flex w-full items-center justify-between gap-3 py-2 text-left" disabled={pickingFood} onclick={() => useEveryday(f)}><span>{f.name}</span><span class="tabular text-base-content/60 text-xs">{f.calories} kcal per 100 g</span></button></li>
						{/each}
					</ul>
				{/if}
			{/if}
			<div class="mt-3"><BarcodeField onfound={findByBarcode} busy={barcodeBusy} message={barcodeMsg} /></div>
			<input class="input mt-3 w-full" type="search" aria-label="Search saved products" placeholder="Find oats, yogurt, bread…" bind:value={productSearch} />
			<ul class="mt-2 max-h-48 divide-y divide-base-content/10 overflow-y-auto">
				{#each matchingProducts as product (product.id)}
					<li><button class="flex w-full items-center justify-between gap-3 py-3 text-left" onclick={() => useProduct(product)}><span><span class="block font-semibold">{product.name}</span><span class="text-base-content/60 text-xs">{round(product.macros.calories)} kcal per {product.basisQuantity} {product.basisUnit}</span></span><span class="flex items-center gap-2">{#if product.rating}<span class="badge {RATING_CLASS[product.rating.label]}" title={product.rating.label}>{product.rating.score}%</span>{/if}<span class="btn btn-soft btn-sm">Use</span></span></button></li>
				{:else}
					<li class="text-base-content/60 py-4 text-sm">{data.products.length ? 'No matching products.' : 'No saved products yet. Pick an everyday food above, or photograph a nutrition panel under Photo → Product label.'}</li>
				{/each}
			</ul>
			{#if matchingFoods.length}
				<p class="mt-3 text-sm font-semibold">Everyday foods <span class="text-base-content/50 font-normal">(real USDA values, per 100 g)</span></p>
				<ul class="divide-base-content/10 mt-1 max-h-48 divide-y overflow-y-auto" aria-label="Matching everyday foods">
					{#each matchingFoods as f (f.name)}
						<li><button type="button" class="flex w-full items-center justify-between gap-3 py-2 text-left" disabled={pickingFood} onclick={() => useEveryday(f)}><span>{f.name}</span><span class="tabular text-base-content/60 text-xs">{f.calories} kcal</span></button></li>
					{/each}
				</ul>
			{/if}
			{/if}
		{/if}

			{#if (mode === 'photo' || mode === 'products') && labelResult}
				<div class="flex flex-col">
					<div class={mode === 'products' ? 'order-2' : 'order-1'}>
				<label class="mt-4 block"><span class="mb-1 block text-sm font-semibold">Product name</span><input class="input w-full" bind:value={name} placeholder="e.g. My usual porridge oats" /></label>
				<p class="badge mt-3" class:badge-warning={!labelResult.fromNutritionLabel} class:badge-ghost={labelResult.fromNutritionLabel}>{labelResult.source === 'usda' ? 'USDA FoodData Central values' : labelResult.source === 'openfoodfacts' ? 'From Open Food Facts' : labelResult.fromNutritionLabel ? 'Read from nutrition panel' : 'Estimated nutrition'}</p>
				{#if shownRating}
					<div class="bg-base-200 rounded-box mt-3 flex items-center justify-between gap-2 p-3">
						<span class="text-sm font-semibold">Ingredient rating</span>
						<RatingBadge rating={shownRating} {name} />
					</div>
				{/if}
				{#if labelResult.ingredients}<p class="text-base-content/65 mt-2 text-sm"><strong>Ingredients:</strong> {labelResult.ingredients}</p>{/if}
				<p class="text-base-content/65 mt-2 text-sm">{labelResult.notes}{labelResult.source === 'usda' ? '' : ' Check the scanned values against the pack.'}</p>
					</div>
					<details class={mode === 'products' ? 'order-3 mt-3' : 'order-2'} open={mode !== 'products'}>
						<summary class={mode === 'products' ? 'cursor-pointer text-sm font-semibold' : 'hidden'}>Check or edit the nutrition</summary>
				<div class="mt-4 grid grid-cols-2 gap-3">
					<label class="floating-label"><span>Label basis</span><input class="input w-full" type="number" min="0.01" step="any" bind:value={labelResult.basisQuantity} /></label>
					<label class="floating-label"><span>Unit</span><select class="select w-full" bind:value={labelResult.basisUnit}><option value="g">grams (g)</option><option value="ml">millilitres (ml)</option><option value="serving">servings</option></select></label>
				</div>
				<p class="text-base-content/60 mt-3 text-xs">Nutrition per {labelResult.basisQuantity} {labelResult.basisUnit}. You can correct these values:</p>
				<div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-3">
					{#each FIELDS as f (f.key)}
						<label class="floating-label"><span>{f.label} ({f.unit})</span><input class="input w-full" type="number" min="0" step="any" bind:value={labelResult.macros[f.key]} /></label>
					{/each}
				</div>
				<button class="btn btn-secondary btn-soft mt-3 w-full" disabled={!productValid || (!!savedProductId && !productChanged)} onclick={saveProductForLater}>{savedProductId ? productChanged ? 'Update saved product' : 'Saved in Products' : 'Save product for next time'}</button>
				<p class="text-base-content/60 mt-1 text-xs">Logging also saves this named product. Its nutrition stays available even if you remove a food log.</p>
					</details>
					<div class={mode === 'products' ? 'order-1' : 'order-3'}>
				<label class="mt-4 block"><span class="mb-1 block text-sm font-semibold">How much did you eat? ({labelResult.basisUnit})</span><input class="input w-full" type="number" min="0.01" step="any" placeholder="Enter your amount" bind:value={labelAmount} /></label>
				{#if labelResult.basisUnit !== 'serving'}
					<div class="mt-2 flex flex-wrap gap-1" aria-label="Quick amounts">
						{#each labelResult.portions ?? [] as q (q.label)}
							<button type="button" class="btn btn-xs btn-primary btn-soft" onclick={() => (labelAmount = round((labelAmount || 0) + q.grams, 1))}>+ {q.label} ({q.grams} {labelResult.basisUnit})</button>
						{/each}
						{#each [50, 100, 150] as g (g)}
							<button type="button" class="btn btn-xs btn-outline border-base-300" onclick={() => (labelAmount = round((labelAmount || 0) + g, 1))}>+ {g} {labelResult.basisUnit}</button>
						{/each}
						{#if labelAmount}<button type="button" class="btn btn-xs btn-ghost" onclick={() => (labelAmount = 0)}>Clear</button>{/if}
					</div>
					{#if labelResult.portions?.length}<p class="text-base-content/50 mt-1 text-xs">Tap to add up amounts, e.g. two taps for two eggs.</p>{/if}
				{/if}
				{#if labelValid}<div class="mt-3"><MacroLine m={labelMacros} /></div>{/if}
					</div>
				</div>
			{/if}

		{#if mode === 'recipe'}
			<label class="mt-4 block">
				<span class="mb-1 block text-sm font-semibold">Recipe</span>
				<select class="select w-full" bind:value={recipeId}>
					<option value="" disabled>Choose a recipe</option>
					{#each sortedRecipes as r (r.id)}<option value={r.id}>{r.title}</option>{/each}
				</select>
			</label>
			{#if chosen}
				<div class="mt-3"><MacroLine m={scaled(chosen.macros, servings)} /></div>
			{:else if data.recipes.length === 0}
				<p class="text-base-content/60 mt-3 text-sm">Recipes appear here once you <a class="link" href="/plans">generate a meal plan</a>.</p>
			{/if}
		{/if}

		{#if mode === 'manual' || (mode === 'describe' && estimate) || (mode === 'photo' && estimate && !labelResult)}
			<div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3">
				<label class="floating-label col-span-2 sm:col-span-3">
					<span>Name</span>
					<input class="input w-full" placeholder="Name" bind:value={name} />
				</label>
				{#if !labelResult || mode !== 'photo'}
				{#each FIELDS as f (f.key)}
					<label class="floating-label">
						<span>{f.label} ({f.unit}) per serving</span>
						<input class="input tabular w-full" type="number" min="0" step="1" placeholder={f.label} bind:value={macros[f.key]} />
					</label>
				{/each}
				{/if}
			</div>
		{/if}

		<div class="modal-action">
			<form method="dialog"><button class="btn btn-ghost">Cancel</button></form>
			{#if mode !== 'recent'}<button class="btn btn-primary" disabled={busy || !canSave} onclick={save}>Log it</button>{/if}
		</div>
		</fieldset>
	</div>
	<form method="dialog" class="modal-backdrop"><button disabled={busy || repeating || savingProduct}>close</button></form>
</dialog>

<dialog bind:this={targetDialog} class="modal modal-bottom sm:modal-middle" oncancel={(event) => { if (savingTarget) event.preventDefault(); }}>
	<div class="modal-box">
		<h3 class="text-2xl font-bold">Calories for {friendlyDate(data.date).toLowerCase()}</h3>
		<p class="text-base-content/65 mt-2 text-sm">Increase or lower this day’s target, for example after exercise. Your usual target is {round(data.day.baseTargets.calories)} kcal. Protein, carbs, fat and fibre targets stay the same.</p>
		<fieldset disabled={savingTarget}>
			<label class="mt-4 block">
				<span class="mb-1 block text-sm font-semibold">Calorie target (kcal)</span>
				<input class="input tabular w-full" type="number" min="1" step="50" bind:value={dayCalories} />
			</label>
			<div class="mt-3 flex flex-wrap gap-2">
				{#each [-250, -100, 100, 250] as amount}
					<button class="btn btn-soft btn-sm" onclick={() => dayCalories = Math.max(1, (dayCalories || data.day.baseTargets.calories) + amount)}>{amount > 0 ? '+' : ''}{amount} kcal</button>
				{/each}
			</div>
			{#if validTarget}<p class="text-base-content/60 mt-2 text-sm">{calorieChange >= 0 ? '+' : ''}{round(calorieChange)} kcal compared with your usual target.</p>{/if}
			<label class="mt-4 block">
				<span class="mb-1 block text-sm font-semibold">Note (optional)</span>
				<input class="input w-full" maxlength="500" bind:value={targetNote} placeholder="Gym session, long walk, quieter day…" />
			</label>
			<div class="modal-action flex-wrap">
				<button class="btn btn-ghost mr-auto" onclick={() => saveTarget(true)}>Reset to usual</button>
				<button class="btn btn-ghost" onclick={() => targetDialog.close()}>Cancel</button>
				<button class="btn btn-primary" disabled={!validTarget || savingTarget} onclick={() => saveTarget()}>{#if savingTarget}<span class="loading loading-spinner loading-sm"></span>{/if}Save</button>
			</div>
		</fieldset>
	</div>
	<form method="dialog" class="modal-backdrop"><button disabled={savingTarget}>close</button></form>
</dialog>
