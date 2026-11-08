<script lang="ts">
 import { startFoodChat } from '#lib/start-food-chat.ts';
	import { invalidateAll } from '$app/navigation';
	import { api, errorText, upload } from '#lib/api.ts';
	import { round } from '#lib/format.ts';
	import { shrinkImage } from '#lib/images.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Product, Scan } from '#lib/types.ts';
	import Icon from '#lib/components/Icon.svelte';
	import RatingBadge from '#lib/components/RatingBadge.svelte';
 import ProductAI from '#lib/components/ProductAI.svelte';
 import { mergeProductSuggestion } from '#lib/product-ai.ts';
	import BarcodeField from '#lib/components/BarcodeField.svelte';
	import { lookupBarcode } from '#lib/barcode.ts';
	import { onMount } from 'svelte';

	let { data } = $props();

	type Sort = 'rating' | 'name' | 'protein' | 'calories';
	let query = $state('');
	let sort = $state<Sort>('rating');
	let open = $state('');

	// Compare like with like: everything per 100 (g/ml) where possible, per serving otherwise.
	const per = (p: Product) => (p.basisUnit === 'serving' ? 1 : 100 / p.basisQuantity);
	const basisLabel = (p: Product) => (p.basisUnit === 'serving' ? 'serving' : `100 ${p.basisUnit}`);

	// What a product still needs before it can be rated (a rating needs both).
	const noIngredients = (p: Product) => !p.ingredients?.trim();
	const noNutrition = (p: Product) => !(p.macros.calories > 0 || p.macros.protein > 0 || p.macros.carbs > 0 || p.macros.fat > 0 || p.macros.fiber > 0);
	const needs = (p: Product) => (p.generic ? (noNutrition(p) ? 'nutrition' : '') : noIngredients(p) && noNutrition(p) ? 'both' : noIngredients(p) ? 'ingredients' : noNutrition(p) ? 'nutrition' : '');
	let filter = $state<'all' | 'needs'>('all');
	const needsCount = $derived(data.products.filter((p) => needs(p)).length);

	const shown = $derived(
		data.products
			.filter((p) => filter === 'all' || needs(p))
			.filter((p) => (p.name + ' ' + (p.barcode ?? '')).toLowerCase().includes(query.trim().toLowerCase()))
			.toSorted((a, b) => {
				switch (sort) {
					case 'name':
						return a.name.localeCompare(b.name);
					case 'protein':
						return b.macros.protein * per(b) - a.macros.protein * per(a);
					case 'calories':
						return a.macros.calories * per(a) - b.macros.calories * per(b);
					default:
						return (b.rating?.score ?? -1) - (a.rating?.score ?? -1);
				}
			})
	);

	// Every part of a product can be edited, then it is re-rated (ratings are never stored).
	type Draft = { notes: string; ai: boolean; generic: boolean; name: string; barcode: string; basisQuantity: number | null; basisUnit: Product['basisUnit']; macros: Record<'calories' | 'protein' | 'carbs' | 'fat' | 'fiber' | 'saturatedFat', number | null>; ingredients: string };
	const toDraft = (p: Product): Draft => ({ notes: p.notes, ai: p.source === 'ai', generic: !!p.generic, name: p.name, barcode: p.barcode ?? '', basisQuantity: p.basisQuantity, basisUnit: p.basisUnit, macros: { ...p.macros, saturatedFat: p.macros.saturatedFat ?? null }, ingredients: p.ingredients ?? '' });
	let draft = $state<Draft | null>(null);
	let savingIng = $state(false);
 function fillProductAI(suggestion: Scan) {
  if (!draft) return;
  const macros = {calories: draft.macros.calories ?? 0, protein: draft.macros.protein ?? 0, carbs: draft.macros.carbs ?? 0, fat: draft.macros.fat ?? 0, fiber: draft.macros.fiber ?? 0, saturatedFat: draft.macros.saturatedFat};
  const merged = mergeProductSuggestion({name: draft.name, ingredients: draft.ingredients, hasNutrition: Object.values(macros).some(v => (v ?? 0) > 0), basisQuantity: draft.basisQuantity ?? 100, basisUnit: draft.basisUnit, macros, source: 'saved', notes: draft.notes}, suggestion);
  draft = {...draft, ai: true, notes: merged.notes ?? draft.notes, ingredients: merged.ingredients, basisQuantity: merged.basisQuantity ?? 100, basisUnit: merged.basisUnit || 'g', macros: {...merged.macros, saturatedFat: merged.macros.saturatedFat ?? null}};
 }
	let ingPhoto = $state<HTMLInputElement>();
	$effect(() => {
		const p = data.products.find((x) => x.id === open);
		draft = p ? toDraft(p) : null;
	});
	const MACRO_FIELDS = [
		['calories', 'Calories'],
		['protein', 'Protein (g)'],
		['carbs', 'Carbs (g)'],
		['fat', 'Fat (g)'],
 ['saturatedFat', 'Saturated fat (g, if known)'],
		['fiber', 'Fibre (g)']
	] as const;
	const dirty = (p: Product) => !!draft && JSON.stringify(draft) !== JSON.stringify(toDraft(p));

	async function saveProduct(p: Product) {
		if (!draft || savingIng) return;
		savingIng = true;
		try {
			await api('/products', {
				body: {
					...p,
					id: p.id,
					rating: undefined,
					name: draft.name.trim(),
					barcode: draft.barcode.replace(/[\s-]/g, ''),
					ingredients: draft.ingredients.trim(),
					generic: draft.generic,
 source: draft.ai ? 'ai' : p.source,
 fromNutritionLabel: draft.ai ? false : p.fromNutritionLabel,
 notes: draft.notes,
					basisQuantity: draft.basisQuantity ?? 0,
					basisUnit: draft.basisUnit,
					macros: Object.fromEntries(Object.entries(draft.macros).map(([k, v]) => [k, k === 'saturatedFat' ? v : v ?? 0]))
				}
			});
			await invalidateAll();
			toast('Saved. The rating has been updated.');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			savingIng = false;
		}
	}

	async function readIngredientsPhoto(event: Event, p: Product) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		savingIng = true;
		try {
			const form = new FormData();
			form.append('photos', await shrinkImage(file), 'ingredients.jpg');
			const scan = await upload<Scan>('/scan/ingredients', form);
			await api(`/products/${p.id}/ingredients`, { method: 'PUT', body: { ingredients: scan.ingredients } });
			await invalidateAll();
			toast('Ingredients read and saved. The rating has been updated.');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			savingIng = false;
		}
	}

	// ---- add a generic whole food (carrots, potatoes): a name and nutrition, no barcode ----
	const emptyNut = () => ({ saturatedFat: null as number | null, calories: null as number | null, protein: null as number | null, carbs: null as number | null, fat: null as number | null, fiber: null as number | null });
	let gen = $state({ name: '', ...emptyNut() });
	let genBusy = $state(false);

	// The built-in everyday foods (loose fruit, veg, potatoes): real USDA numbers, per 100 g, raw.
	type Food = { portions: { label: string; grams: number }[]; name: string; aliases: string[]; category: string; calories: number; protein: number; carbs: number; fat: number; fiber: number; fdcId: number; usda: string };
	let foods = $state<Food[]>([]);
	let foodsSource = $state('');
	let picked = $state<Food | null>(null);
	onMount(async () => {
		try {
			const r = await api<{ foods: Food[]; source: string }>('/foods');
			foods = r.foods;
			foodsSource = r.source;
		} catch {
			/* the form still works by typing */
		}
	});
	const POPULAR = ['Eggs (boiled)', 'Chicken breast (cooked)', 'Potatoes (white)', 'Tomatoes', 'Grapes', 'Bananas', 'Apples', 'Carrots', 'Onions', 'Bacon (cooked)'];
	const popular = $derived(POPULAR.map((n) => foods.find((f) => f.name === n)).filter((f): f is Food => !!f));
	const suggestions = $derived.by(() => {
		const q = gen.name.trim().toLowerCase();
		if (!q) return [];
		const rank = (f: Food) => (f.name.toLowerCase().startsWith(q) ? 0 : f.name.toLowerCase().includes(q) ? 1 : f.aliases.some((a) => a.toLowerCase().startsWith(q)) ? 2 : f.aliases.some((a) => a.toLowerCase().includes(q)) ? 3 : -1);
		return foods.filter((f) => rank(f) >= 0).toSorted((a, b) => rank(a) - rank(b)).slice(0, 8);
	});
	function pickFood(f: Food) {
		picked = f;
		gen.name = f.name;
		gen.calories = f.calories;
		gen.protein = f.protein;
		gen.carbs = f.carbs;
		gen.fat = f.fat;
		gen.fiber = f.fiber;
 gen.saturatedFat = null;
	}
	// Still the list's own numbers (not edited since you picked it)?
	const fromList = $derived(
		!!picked && gen.name.trim() === picked.name && gen.calories === picked.calories && gen.protein === picked.protein && gen.carbs === picked.carbs && gen.fat === picked.fat && gen.fiber === picked.fiber
	);
	const genValid = $derived(!!gen.name.trim() && [gen.calories, gen.protein, gen.carbs, gen.fat, gen.fiber].some((v) => (v ?? 0) > 0));

	async function estimateGeneric() {
		if (!gen.name.trim() || genBusy) return;
		genBusy = true;
		try {
			const est = await api<{ macros: Record<string, number> }>('/log/estimate', { body: { text: `100 g ${gen.name.trim()}, raw` } });
			gen.calories = est.macros.calories;
			gen.protein = est.macros.protein;
			gen.carbs = est.macros.carbs;
			gen.fat = est.macros.fat;
			gen.fiber = est.macros.fiber;
 gen.saturatedFat = est.macros.saturatedFat ?? null;
			toast('Estimated by the model for 100 g. Check the values against a trusted source.', 'info');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			genBusy = false;
		}
	}

	async function addGeneric() {
		if (!genValid || genBusy) return;
		genBusy = true;
		try {
			const saved = await api<Product>('/products', {
				body: {
					name: gen.name.trim(),
					generic: true,
					basisQuantity: 100,
					basisUnit: 'g',
					fromNutritionLabel: fromList, // list values are reference data; typed or estimated ones are not
					source: fromList ? 'usda' : '',
					portions: fromList && picked ? picked.portions : [],
					confidence: fromList ? 'medium' : 'low',
					notes: fromList && picked ? `Per 100 g, raw. From USDA FoodData Central (SR Legacy #${picked.fdcId}): ${picked.usda}.` : '',
					macros: { calories: gen.calories ?? 0, protein: gen.protein ?? 0, carbs: gen.carbs ?? 0, fat: gen.fat ?? 0, saturatedFat: gen.saturatedFat, fiber: gen.fiber ?? 0 }
				}
			});
			await invalidateAll();
			gen = { name: '', ...emptyNut() };
			picked = null;
			filter = 'all';
			query = '';
			open = saved.id;
			toast(`Added ${saved.name}`);
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			genBusy = false;
		}
	}

	// ---- export / import ----
	const hasNutrition = (p: Product) => !noNutrition(p) && p.basisQuantity > 0;
	const canExport = (p: Product) => (p.generic ? hasNutrition(p) : !!p.barcode && !noIngredients(p) && hasNutrition(p));
	const exportable = $derived(data.products.filter(canExport));
	const leftOut = $derived(
		data.products
			.filter((p) => !canExport(p))
			.map((p) => ({ name: p.name, why: [!p.generic && !p.barcode ? 'no barcode' : '', !p.generic && noIngredients(p) ? 'no ingredients' : '', !hasNutrition(p) ? 'no nutrition' : ''].filter(Boolean).join(', ') }))
	);

	type ImportResult = { total: number; add: { name: string; barcode: string }[]; duplicates: { name: string; barcode?: string; reason: string }[]; invalid: { name: string; barcode?: string; reason: string }[]; applied: boolean };
	let importBody = $state<unknown>(null);
	let importPlan = $state<ImportResult | null>(null);
	let importFile = $state<HTMLInputElement>();
	let importDialog: HTMLDialogElement;
	let importing = $state(false);

	async function chooseImport(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file) return;
		importing = true;
		try {
			if (file.size > 8 * 1024 * 1024) throw new Error('That file is over 8 MB.');
			let body: unknown;
			try {
				body = JSON.parse(await file.text());
			} catch {
				throw new Error('Choose a Larder products file (.json) exported from this page.');
			}
			importPlan = await api<ImportResult>('/products/import/preview', { body }); // a preview: nothing changes yet
			importBody = body;
			importDialog.showModal();
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			importing = false;
		}
	}

	async function confirmImport() {
		if (!importBody || importing) return;
		importing = true;
		try {
			const done = await api<ImportResult>('/products/import', { body: importBody });
			importDialog.close();
			importBody = null;
			await invalidateAll();
			toast(done.add.length ? `Imported ${done.add.length} product${done.add.length === 1 ? '' : 's'}. Nothing you already had was changed.` : 'Nothing new to import.', 'info');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			importing = false;
		}
	}

	// A link to /products#generic opens the generic-food form.
	onMount(() => {
		if (location.hash === '#generic') {
			const el = document.getElementById('generic');
			el?.setAttribute('open', '');
			el?.scrollIntoView();
		}
	});

	let adding = $state(false);
	let addMsg = $state('');
	async function addByBarcode(code: string) {
		adding = true;
		addMsg = '';
		try {
			const found = await lookupBarcode(code);
			if (found.source === 'saved') {
				open = found.product.id;
				query = '';
				toast(`Already saved as ${found.product.name}`, 'info');
				return;
			}
			const saved = await api<Product>('/products', { body: found.product });
			await invalidateAll();
			query = '';
			open = saved.id;
			toast(`Added ${saved.name}`);
		} catch (e) {
			addMsg = errorText(e);
		} finally {
			adding = false;
		}
	}

	async function remove(p: Product) {
		if (!confirm(`Delete "${p.name}"? Food you already logged is not affected.`)) return;
		try {
			await api(`/products/${p.id}`, { method: 'DELETE' });
			await invalidateAll();
			toast('Product deleted');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}
</script>

<svelte:head><title>Products · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Products</h1>
<p class="text-base-content/70 mt-1 text-sm">
	Every product you've scanned or saved, with nutrition and an ingredient rating. Add one by barcode above (saved products are found offline; other barcodes need online lookup turned on in Settings), or photograph a label under Today → Log food → Photo → Product label.
</p>

<div class="mt-5 max-w-3xl">
	<p class="mb-1 text-sm font-semibold">Add by barcode</p>
	<BarcodeField onfound={addByBarcode} busy={adding} label="Add" message={addMsg} />
</div>

<details class="border-base-300 rounded-box mt-4 max-w-3xl border" id="generic">
	<summary class="cursor-pointer p-3 font-semibold">Add a generic food <span class="text-base-content/60 font-normal">(carrots, potatoes… no barcode needed)</span></summary>
	<div class="px-3 pb-3">
		<p class="text-base-content/70 text-sm">For loose whole foods with no pack, like potatoes, grapes or tomatoes. Pick one from the list (real USDA values per 100 g) or type your own nutrition; the food counts as its own single ingredient, so it is rated like anything else.</p>
		<input class="input mt-2 w-full" placeholder="Start typing: potatoes, grapes, tomatoes…" aria-label="Food name" maxlength="120" autocomplete="off" bind:value={gen.name} />
		{#if !gen.name.trim() && popular.length}
			<div class="mt-2 flex flex-wrap gap-1" aria-label="Common foods">
				{#each popular as f (f.name)}<button type="button" class="btn btn-xs btn-outline border-base-300 rounded-full" onclick={() => pickFood(f)}>{f.name}</button>{/each}
			</div>
			<p class="text-base-content/50 mt-1 text-xs">Tap a common food to fill in its nutrition, or type any name.</p>
		{:else if suggestions.length && !(picked && gen.name.trim() === picked.name)}
			<ul class="border-base-300 rounded-box divide-base-content/10 mt-1 max-h-52 divide-y overflow-y-auto border" aria-label="Matching foods">
				{#each suggestions as f (f.name)}
					<li><button type="button" class="flex w-full items-center justify-between gap-3 p-2 text-left text-sm" onclick={() => pickFood(f)}><span>{f.name}</span><span class="tabular text-base-content/60 text-xs">{f.calories} kcal per 100 g</span></button></li>
				{/each}
			</ul>
		{/if}
		{#if fromList && picked}<p class="text-base-content/60 mt-1 text-xs">Per 100 g, raw. Source: {foodsSource}: “{picked.usda}”.</p>{/if}
		<div class="mt-2 grid grid-cols-3 gap-2 sm:grid-cols-5">
			{#each MACRO_FIELDS as [key, label] (key)}
				<label class="text-xs">{label}<input class="input input-sm tabular mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={gen[key]} /></label>
			{/each}
		</div>
		<div class="mt-3 flex flex-wrap gap-2">
			<button class="btn btn-sm btn-primary" disabled={!genValid || genBusy} onclick={addGeneric}>Add food</button>
			<button class="btn btn-sm" disabled={!gen.name.trim() || genBusy} onclick={estimateGeneric} title="Asks your model for typical values per 100 g">
				{#if genBusy}<span class="loading loading-spinner loading-sm"></span>{/if}Not on the list? Fill it in with the model
			</button>
		</div>
		<p class="text-base-content/50 mt-2 text-xs">Foods from the list use real USDA values. The model's numbers are only estimates, so check them before relying on them.</p>
	</div>
</details>

<details class="border-base-300 rounded-box mt-3 max-w-3xl border">
	<summary class="cursor-pointer p-3 font-semibold">Export or import products</summary>
	<div class="px-3 pb-3 text-sm">
		<p class="font-semibold">Export</p>
		<p class="text-base-content/70">Saves {exportable.length} of your {data.products.length} products to a file you can keep or import into another account. Only complete products are included: a barcode, ingredients and nutrition (generic whole foods need only nutrition).</p>
		{#if leftOut.length}
			<details class="mt-1"><summary class="text-base-content/60 cursor-pointer text-xs">{leftOut.length} left out</summary>
				<ul class="text-base-content/60 mt-1 text-xs">{#each leftOut as l (l.name)}<li>{l.name}: {l.why}</li>{/each}</ul>
			</details>
		{/if}
		<a class="btn btn-sm btn-primary mt-2 {exportable.length ? '' : 'btn-disabled'}" href="/api/products/export" download aria-disabled={!exportable.length}>Download products file</a>

		<p class="mt-5 font-semibold">Import</p>
		<p class="text-base-content/70">Choose a products file. You'll see what would be added first. <strong>Nothing you already have is ever changed or replaced</strong>: a product with the same barcode or name is skipped, and anything without a barcode, ingredients and nutrition is rejected (generic whole foods need only nutrition).</p>
		<input bind:this={importFile} type="file" accept="application/json,.json" class="hidden" onchange={chooseImport} />
		<button class="btn btn-sm mt-2" disabled={importing} onclick={() => importFile?.click()}>{#if importing}<span class="loading loading-spinner loading-sm"></span>{/if}Choose a file to import…</button>
	</div>
</details>

<dialog bind:this={importDialog} class="modal modal-bottom sm:modal-middle" aria-label="Import products">
	<div class="modal-box max-h-[90dvh]">
		<h3 class="text-xl font-bold">Import products</h3>
		{#if importPlan}
			<p class="mt-2 text-sm">{importPlan.total} in the file. <strong>{importPlan.add.length} would be added</strong>, {importPlan.duplicates.length} skipped as already yours, {importPlan.invalid.length} rejected.</p>
			<p class="alert alert-info alert-soft mt-2 text-sm">Nothing you already have is changed or replaced.</p>
			{#if importPlan.add.length}
				<details class="mt-3" open><summary class="cursor-pointer text-sm font-semibold">Will be added ({importPlan.add.length})</summary>
					<ul class="mt-1 max-h-40 overflow-y-auto text-sm">{#each importPlan.add as it, i (i)}<li>{it.name}{it.barcode ? ` · ${it.barcode}` : ' · generic'}</li>{/each}</ul>
				</details>
			{/if}
			{#if importPlan.duplicates.length}
				<details class="mt-3"><summary class="cursor-pointer text-sm font-semibold">Skipped: already yours ({importPlan.duplicates.length})</summary>
					<ul class="text-base-content/70 mt-1 max-h-40 overflow-y-auto text-sm">{#each importPlan.duplicates as d, i (i)}<li>{d.name}: {d.reason}</li>{/each}</ul>
				</details>
			{/if}
			{#if importPlan.invalid.length}
				<details class="mt-3" open={!importPlan.add.length}><summary class="cursor-pointer text-sm font-semibold">Rejected ({importPlan.invalid.length})</summary>
					<ul class="text-base-content/70 mt-1 max-h-40 overflow-y-auto text-sm">{#each importPlan.invalid as d, i (i)}<li>{d.name}: {d.reason}</li>{/each}</ul>
				</details>
			{/if}
		{/if}
		<div class="modal-action">
			<button class="btn btn-primary" disabled={!importPlan?.add.length || importing} onclick={confirmImport}>
				{#if importing}<span class="loading loading-spinner loading-sm"></span>{/if}Import {importPlan?.add.length ?? 0} product{importPlan?.add.length === 1 ? '' : 's'}
			</button>
			<form method="dialog"><button class="btn">Cancel</button></form>
		</div>
	</div>
	<form method="dialog" class="modal-backdrop"><button aria-label="Close">close</button></form>
</dialog>

<div class="mt-5 flex max-w-3xl flex-wrap gap-2" role="group" aria-label="Show">
	<button class="btn btn-sm rounded-full {filter === 'all' ? 'btn-primary' : 'btn-outline border-base-300'}" aria-pressed={filter === 'all'} onclick={() => (filter = 'all')}>All ({data.products.length})</button>
	<button class="btn btn-sm rounded-full {filter === 'needs' ? 'btn-primary' : 'btn-outline border-base-300'}" aria-pressed={filter === 'needs'} onclick={() => (filter = 'needs')}>Need ingredients or nutrition ({needsCount})</button>
</div>

<div class="mt-3 flex max-w-3xl flex-col gap-2 sm:flex-row">
	<input class="input w-full" type="search" aria-label="Search products" placeholder="Search name or barcode…" bind:value={query} />
	<select class="select sm:w-48" aria-label="Sort by" bind:value={sort}>
		<option value="rating">Best rated</option>
		<option value="name">Name</option>
		<option value="protein">Most protein</option>
		<option value="calories">Fewest calories</option>
	</select>
</div>

<ul class="mt-4 flex max-w-3xl flex-col gap-3">
	{#each shown as p (p.id)}
		<li class="border-base-300 rounded-box border">
			<div class="flex items-center gap-2 pr-4">
				<button class="flex min-w-0 flex-1 items-center justify-between gap-3 p-4 text-left" aria-expanded={open === p.id} onclick={() => (open = open === p.id ? '' : p.id)}>
					<span class="min-w-0">
						<span class="block truncate font-semibold">{p.name}</span>
						<span class="text-base-content/60 tabular text-xs">
							per {basisLabel(p)}: {round(p.macros.calories * per(p))} kcal · P {round(p.macros.protein * per(p), 1)} · C {round(p.macros.carbs * per(p), 1)} · F {round(p.macros.fat * per(p), 1)}{#if p.macros.saturatedFat != null} · Saturates {round(p.macros.saturatedFat * per(p), 1)} g{/if} · Fibre {round(p.macros.fiber * per(p), 1)}
						</span>
					</span>
				</button>
				{#if p.rating}
					<RatingBadge rating={p.rating} name={p.name} />
				{:else}
					<span class="badge badge-warning badge-soft shrink-0" title="A rating needs both the ingredients and the nutrition">Needs {needs(p) || 'checking'}</span>
				{/if}
			</div>
			{#if open === p.id}
				<div class="border-base-300 border-t p-4 text-sm">
					{#if p.rating}
						<p class="font-semibold capitalize">{p.rating.label} · {p.rating.score}% <RatingBadge rating={p.rating} name={p.name} variant="link" /></p>
						<p class="text-base-content/70 mt-1">
							{#each p.rating.factors.filter((f) => f.points < 0).slice(0, 3) as f, i (i)}{i ? ', ' : 'Marked down for: '}{f.text}{/each}{#if !p.rating.factors.some((f) => f.points < 0)}Nothing marks it down.{/if}
						</p>
					{:else}
						<p class="text-base-content/70">No rating yet: a rating needs both the ingredients and the nutrition. Still needed: {needs(p) === 'both' ? 'ingredients and nutrition' : needs(p)}. Fill them in below.</p>
					{/if}
					{#if draft}
 <button class="btn btn-sm mt-3" onclick={() => startFoodChat({kind: 'product', productId: p.id, barcode: draft?.barcode, context: JSON.stringify({name: draft?.name, ingredients: draft?.ingredients, basisQuantity: draft?.basisQuantity, basisUnit: draft?.basisUnit, macros: draft?.macros})})}>Discuss this product in Food chat</button>
 <ProductAI name={draft.name} onaccept={(suggestion) => fillProductAI(suggestion)} />
						<div class="mt-3 grid gap-3">
							<div class="grid gap-3 sm:grid-cols-2">
								<label class="text-xs font-semibold">Name<input class="input input-sm mt-1 w-full text-sm font-normal" bind:value={draft.name} /></label>
								<label class="text-xs font-semibold">Barcode<input class="input input-sm mt-1 w-full font-mono text-sm font-normal" inputmode="numeric" placeholder="optional" bind:value={draft.barcode} /></label>
							</div>
							<div>
								<p class="text-xs font-semibold">
									Nutrition per
									<input class="input input-xs tabular mx-1 w-16 font-normal" type="number" min="0" step="any" aria-label="Amount" bind:value={draft.basisQuantity} />
									<select class="select select-xs w-auto font-normal" aria-label="Unit" bind:value={draft.basisUnit}><option value="g">g</option><option value="ml">ml</option><option value="serving">serving</option></select>
								</p>
								<div class="mt-2 grid grid-cols-3 gap-2 sm:grid-cols-5">
									{#each MACRO_FIELDS as [key, label] (key)}
										<label class="text-xs">{label}<input class="input input-sm tabular mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={draft.macros[key]} /></label>
									{/each}
								</div>
							</div>
							<label class="flex cursor-pointer items-start gap-2 text-xs">
								<input type="checkbox" class="checkbox checkbox-sm mt-0.5" bind:checked={draft.generic} />
								<span><span class="font-semibold">Generic whole food</span> (carrots, potatoes, an apple): no barcode or ingredients label needed, and it counts as its own single ingredient.</span>
							</label>
							<div>
								<label class="text-xs font-semibold" for="ing-{p.id}">Ingredients{draft.generic ? ' (optional)' : ''}</label>
								<textarea id="ing-{p.id}" class="textarea mt-1 w-full text-sm" rows="3" placeholder={draft.generic ? 'Leave empty: the food itself is the ingredient.' : 'No ingredients yet. Type or paste them, or photograph the pack.'} bind:value={draft.ingredients}></textarea>
							</div>
						</div>
						<div class="mt-3 flex flex-wrap gap-2">
							{#if dirty(p)}
								<button class="btn btn-sm btn-primary" disabled={savingIng || !draft.name.trim()} onclick={() => saveProduct(p)}>Save changes</button>
								<button class="btn btn-sm btn-ghost" disabled={savingIng} onclick={() => (draft = toDraft(p))}>Undo</button>
							{/if}
							<input bind:this={ingPhoto} type="file" accept="image/*" capture="environment" class="hidden" onchange={(e) => readIngredientsPhoto(e, p)} />
							<button class="btn btn-sm" disabled={savingIng} onclick={() => ingPhoto?.click()}><Icon name="camera" size={16} />Photograph ingredients</button>
						</div>
					{/if}
					<p class="text-base-content/50 mt-3 text-xs">
						{p.source === 'ai' ? 'AI estimate — not verified against a package' : p.source === 'usda' ? 'Reference values from USDA FoodData Central' : p.fromNutritionLabel ? 'Read from a nutrition panel' : 'Estimated nutrition'}. The rating is a rough guide from ingredients and macros; it doesn't know salt or sugar totals. Not medical advice.
					</p>
					<button class="btn btn-ghost btn-sm text-error mt-2" onclick={() => remove(p)}><Icon name="trash" size={16} />Delete</button>
				</div>
			{/if}
		</li>
	{:else}
		<li class="text-base-content/60 py-8 text-center text-sm">
			{data.products.length ? 'No matching products.' : 'No products yet. Photograph a nutrition panel under Today → Log food → Photo → Product label.'}
		</li>
	{/each}
</ul>
