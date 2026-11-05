<script lang="ts">
	import { cap } from '#lib/format.ts';
	import { MEAL_TYPES } from '#lib/types.ts';
	import MacroStripe from '#lib/components/MacroStripe.svelte';
	import MealBadge from '#lib/components/MealBadge.svelte';
	import Stars from '#lib/components/Stars.svelte';

	let { data } = $props();

	let q = $state('');
	let meal = $state('all');
	let show = $state<'all' | 'unrated' | 'loved'>('all');

	const filtered = $derived(
		data.recipes.filter((r) => {
			if (meal !== 'all' && r.mealType !== meal) return false;
			if (show === 'unrated' && r.ratingCount > 0) return false;
			if (show === 'loved' && r.avgRating < 4) return false;
			const s = q.trim().toLowerCase();
			return !s || r.title.toLowerCase().includes(s) || r.ingredients.some((i) => i.name.toLowerCase().includes(s));
		})
	);
	const fb = $derived(data.feedback);
	const unrated = $derived(data.recipes.filter((r) => r.ratingCount === 0).length);
</script>

<svelte:head><title>Recipes · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Recipes</h1>

<!-- The feedback loop, made visible -->
<section class="bg-base-200 rounded-box mt-6 p-5">
	<h2 class="text-xl font-bold">What your ratings are teaching it</h2>
	{#if fb.ratingCount === 0}
		<p class="text-base-content/70 mt-1">
			Rate a few meals and new plans will lean toward what you enjoy and away from what you don't.
		</p>
	{:else}
		<div class="mt-3 grid gap-4 sm:grid-cols-2">
			<div>
				<p class="text-sm font-semibold">More of</p>
				<div class="mt-1.5 flex flex-wrap gap-1.5">
					{#each [...new Set([...(fb.likedIngredients ?? []), ...(fb.likedTags ?? [])])] as t (t)}
						<span class="badge badge-success badge-soft">{t}</span>
					{:else}
						<span class="text-base-content/50 text-sm">Not enough 4–5 star ratings yet</span>
					{/each}
				</div>
			</div>
			<div>
				<p class="text-sm font-semibold">Less of</p>
				<div class="mt-1.5 flex flex-wrap gap-1.5">
					{#each [...(fb.dislikedIngredients ?? []), ...(fb.disliked ?? []).map((r) => r.title)] as t (t)}
						<span class="badge badge-error badge-soft">{t}</span>
					{:else}
						<span class="text-base-content/50 text-sm">Nothing yet</span>
					{/each}
				</div>
			</div>
		</div>
		<p class="text-base-content/60 mt-3 text-sm">
			Based on {fb.ratingCount} rating{fb.ratingCount === 1 ? '' : 's'}.{#if unrated}&nbsp;{unrated} still unrated.{/if}
		</p>
	{/if}
</section>

<div class="mt-6 flex flex-wrap gap-3">
	<input class="input w-full sm:w-64" type="search" placeholder="Search recipes or ingredients" bind:value={q} />
	<select class="select w-auto" bind:value={meal} aria-label="Meal type">
		<option value="all">All meals</option>
		{#each MEAL_TYPES as t (t)}<option value={t}>{cap(t)}</option>{/each}
	</select>
	<div class="join">
		{#each [['all', 'All'], ['unrated', 'Unrated'], ['loved', 'Loved']] as [v, label] (v)}
			<input class="join-item btn btn-sm h-10" type="radio" name="show" aria-label={label} checked={show === v} onchange={() => (show = v as typeof show)} />
		{/each}
	</div>
</div>

{#if data.recipes.length === 0}
	<p class="text-base-content/60 mt-6">Recipes from your meal plans collect here. <a class="link" href="/plans">Generate a plan</a> to get started.</p>
{:else if filtered.length === 0}
	<p class="text-base-content/60 mt-6">No recipes match. Try clearing the search or filters.</p>
{:else}
	<ul class="divide-base-content/10 border-base-content/10 mt-5 divide-y border-t">
		{#each filtered as r (r.id)}
			<li>
				<a href="/recipes/{r.id}" class="group flex items-center gap-4 py-4">
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-3">
							<MealBadge type={r.mealType} />
							{#if r.source === 'pantry'}<span class="text-base-content/55 text-xs">from pantry</span>{/if}
						</div>
						<p class="font-display mt-0.5 truncate text-lg leading-snug font-bold group-hover:underline">{r.title}</p>
						<div class="mt-2 max-w-72"><MacroStripe m={r.macros} height={5} /></div>
					</div>
					<div class="flex flex-col items-end gap-1">
						<p class="tabular"><span class="font-display text-lg font-bold">{Math.round(r.macros.calories)}</span> <span class="text-base-content/55 text-xs">kcal</span></p>
						{#if r.ratingCount}<Stars readonly size="sm" value={r.avgRating} />{:else}<span class="text-base-content/45 text-xs">Not rated</span>{/if}
					</div>
				</a>
			</li>
		{/each}
	</ul>
{/if}
