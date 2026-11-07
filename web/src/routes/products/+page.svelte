<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { api, errorText, upload } from '#lib/api.ts';
	import { round } from '#lib/format.ts';
	import { shrinkImage } from '#lib/images.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { Product, Scan } from '#lib/types.ts';
	import Icon from '#lib/components/Icon.svelte';
	import RatingBadge from '#lib/components/RatingBadge.svelte';
	import BarcodeField from '#lib/components/BarcodeField.svelte';
	import { lookupBarcode } from '#lib/barcode.ts';

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
	const needs = (p: Product) => (noIngredients(p) && noNutrition(p) ? 'both' : noIngredients(p) ? 'ingredients' : noNutrition(p) ? 'nutrition' : '');
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
	type Draft = { name: string; barcode: string; basisQuantity: number | null; basisUnit: Product['basisUnit']; macros: Record<'calories' | 'protein' | 'carbs' | 'fat' | 'fiber', number | null>; ingredients: string };
	const toDraft = (p: Product): Draft => ({ name: p.name, barcode: p.barcode ?? '', basisQuantity: p.basisQuantity, basisUnit: p.basisUnit, macros: { ...p.macros }, ingredients: p.ingredients ?? '' });
	let draft = $state<Draft | null>(null);
	let savingIng = $state(false);
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
					basisQuantity: draft.basisQuantity ?? 0,
					basisUnit: draft.basisUnit,
					macros: Object.fromEntries(Object.entries(draft.macros).map(([k, v]) => [k, v ?? 0]))
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
							per {basisLabel(p)}: {round(p.macros.calories * per(p))} kcal · P {round(p.macros.protein * per(p), 1)} · C {round(p.macros.carbs * per(p), 1)} · F {round(p.macros.fat * per(p), 1)} · Fibre {round(p.macros.fiber * per(p), 1)}
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
							<div>
								<label class="text-xs font-semibold" for="ing-{p.id}">Ingredients</label>
								<textarea id="ing-{p.id}" class="textarea mt-1 w-full text-sm" rows="3" placeholder="No ingredients yet. Type or paste them, or photograph the pack." bind:value={draft.ingredients}></textarea>
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
						{p.fromNutritionLabel ? 'Read from a nutrition panel' : 'Estimated nutrition'}. The rating is a rough guide from ingredients and macros; it doesn't know salt or sugar totals. Not medical advice.
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
