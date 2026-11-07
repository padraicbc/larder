<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { api, errorText } from '#lib/api.ts';
	import { cap, round } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { MEAL_TYPES, type SavedMeal } from '#lib/types.ts';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	let editing = $state('');
	let name = $state('');
	let slot = $state<SavedMeal['mealType']>('breakfast');

	function edit(m: SavedMeal) {
		editing = m.id;
		name = m.name;
		slot = m.mealType;
	}

	async function save(m: SavedMeal) {
		try {
			await api(`/meals/${m.id}`, { method: 'PUT', body: { name, mealType: slot } });
			editing = '';
			await invalidateAll();
			toast('Meal updated');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function remove(m: SavedMeal) {
		if (!confirm(`Delete “${m.name}”? Meals you already logged and plans that used it are not affected.`)) return;
		try {
			await api(`/meals/${m.id}`, { method: 'DELETE' });
			await invalidateAll();
			toast('Meal deleted');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}
</script>

<svelte:head><title>Meals · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Your meals</h1>
<p class="text-base-content/70 mt-1 text-sm">
	Named groups of foods, like “Usual breakfast”. Log one again from Today → Log food → Recent, or fix one to a slot when you
	<a class="link" href="/plans">plan</a>. Make one on Today with <em>Group as meal</em>.
</p>

<ul class="mt-5 flex max-w-3xl flex-col gap-3">
	{#each data.meals as m (m.id)}
		<li class="border-base-300 rounded-box border p-4">
			{#if editing === m.id}
				<div class="flex flex-col gap-2 sm:flex-row">
					<input class="input w-full" aria-label="Meal name" bind:value={name} />
					<select class="select sm:w-40" aria-label="Usual slot" bind:value={slot}>
						{#each MEAL_TYPES as t (t)}<option value={t}>{cap(t)}</option>{/each}
					</select>
				</div>
				<div class="mt-2 flex gap-2">
					<button class="btn btn-primary btn-sm" disabled={!name.trim()} onclick={() => save(m)}>Save</button>
					<button class="btn btn-ghost btn-sm" onclick={() => (editing = '')}>Cancel</button>
				</div>
			{:else}
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0">
						<p class="font-semibold">{m.name} <span class="badge badge-ghost badge-sm">{m.mealType}</span></p>
						<p class="text-base-content/70 mt-1 text-sm">{m.items.map((i) => `${i.quantity} ${i.name}`).join(', ')}</p>
						<p class="tabular text-base-content/60 mt-1 text-xs">
							{round(m.macros.calories)} kcal · P {round(m.macros.protein, 1)} · C {round(m.macros.carbs, 1)} · F {round(m.macros.fat, 1)} · Fibre {round(m.macros.fiber, 1)}
						</p>
					</div>
					<div class="flex shrink-0">
						<button class="btn btn-ghost btn-sm" onclick={() => edit(m)}>Edit</button>
						<button class="btn btn-ghost btn-square btn-sm" aria-label="Delete {m.name}" onclick={() => remove(m)}><Icon name="trash" size={16} /></button>
					</div>
				</div>
			{/if}
		</li>
	{:else}
		<li class="text-base-content/60 py-8 text-center text-sm">No saved meals yet. On Today, tap <em>Group as meal</em> on a few logged foods.</li>
	{/each}
</ul>
