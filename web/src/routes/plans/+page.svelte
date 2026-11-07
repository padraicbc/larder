<script lang="ts">
	import { goto } from '$app/navigation';
	import { untrack } from 'svelte';
	import { api, errorText } from '#lib/api.ts';
	import { cap } from '#lib/format.ts';
	import { MEAL_TYPES } from '#lib/types.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { PlanDetail } from '#lib/types.ts';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	let days = $state(3);
	let notes = $state('');
	let busy = $state(false);
	let error = $state('');

	// Slots fixed to a saved meal (slot -> meal id). Remembered on this device.
	const KEY = 'larder.fixedSlots';
	function loadFixed(): Record<string, string> {
		try {
			return JSON.parse(localStorage.getItem(KEY) ?? '{}');
		} catch {
			return {};
		}
	}
	let fixed = $state<Record<string, string>>({});
	$effect(() => {
		const ids = new Set(data.meals.map((m) => m.id));
		fixed = Object.fromEntries(Object.entries(untrack(loadFixed)).filter(([, id]) => ids.has(id)));
	});
	function setFixed(slot: string, id: string) {
		if (id) fixed[slot] = id;
		else delete fixed[slot];
		try {
			localStorage.setItem(KEY, JSON.stringify(fixed));
		} catch {}
	}
	const fixedCount = $derived(Object.keys(fixed).length);

	const constraints = $derived(
		[...data.profile.diets, ...data.profile.allergies.map((a) => `no ${a}`), ...data.profile.excluded.map((e) => `no ${e}`)]
	);

	async function generate() {
		busy = true;
		error = '';
		try {
			const res = await api<PlanDetail>('/plans', { body: { days, notes, ...(fixedCount ? { fixed } : {}) } });
			toast('Plan ready');
			await goto(`/plans/${res.plan.id}`);
		} catch (e) {
			error = errorText(e);
		} finally {
			busy = false;
		}
	}

	const fmt = (iso: string) => new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' });
</script>

<svelte:head><title>Meal plans · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Meal plans</h1>

<section class="bg-base-200 rounded-box mt-6 p-5 sm:p-6">
	<h2 class="text-xl font-bold">Plan the next few days</h2>
	<p class="text-base-content/70 mt-1 text-sm">
		Built from your targets of {data.profile.targets.calories} kcal and {data.profile.targets.protein}g protein,
		{data.profile.mealsPerDay} meals a day{data.profile.householdSize > 1 ? ` for ${data.profile.householdSize} people` : ''}.
		<a href="/profile" class="link">Change</a>
	</p>
	{#if constraints.length}
		<div class="mt-3 flex flex-wrap gap-1.5">
			{#each constraints as c (c)}<span class="badge badge-sm badge-primary badge-soft">{c}</span>{/each}
		</div>
	{/if}

	<div class="mt-5 grid gap-4 sm:grid-cols-[12rem_1fr]">
		<label>
			<span class="mb-1 block text-sm font-semibold">Days</span>
			<div class="join w-full">
				{#each [1, 3, 5, 7] as n (n)}
					<input
						class="join-item btn btn-sm flex-1"
						type="radio"
						name="days"
						aria-label={String(n)}
						checked={days === n}
						onchange={() => (days = n)}
					/>
				{/each}
			</div>
		</label>
		<label>
			<span class="mb-1 block text-sm font-semibold">Anything specific this time?</span>
			<input class="input w-full" placeholder="Quick lunches I can take to work, one fish night" bind:value={notes} />
		</label>
	</div>

	<div class="mt-5">
		<p class="mb-1 text-sm font-semibold">Meals you have the same every day</p>
		{#if data.meals.length === 0}
			<p class="text-base-content/60 text-sm">
				Save a meal first: on Today, tap <em>Group as meal</em> on a few logged foods, give it a name, and it will show up here to fix to breakfast, lunch, dinner or a snack.
			</p>
		{:else}
			<div class="grid gap-2 sm:grid-cols-2">
				{#each MEAL_TYPES as slot (slot)}
					<label class="flex items-center gap-2">
						<span class="w-24 shrink-0 text-sm">{cap(slot)}</span>
						<select class="select select-sm w-full" value={fixed[slot] ?? ''} onchange={(e) => setFixed(slot, e.currentTarget.value)}>
							<option value="">Planned for me</option>
							{#each data.meals as m (m.id)}<option value={m.id}>{m.name} ({Math.round(m.macros.calories)} kcal)</option>{/each}
						</select>
					</label>
				{/each}
			</div>
			{#if fixedCount}<p class="text-base-content/60 mt-1 text-xs">These go into every day as they are, and only the other meals are planned around what’s left of your targets.</p>{/if}
		{/if}
	</div>

	{#if error}
		<div role="alert" class="alert alert-error alert-soft mt-4">
			<Icon name="alert" />
			<span>{error} <a class="link" href="/settings">Check model settings</a></span>
		</div>
	{/if}

	<button class="btn btn-primary mt-5 w-full sm:w-auto" disabled={busy} onclick={generate}>
		{#if busy}
			<span class="loading loading-spinner loading-sm"></span>Writing your plan…
		{:else}
			<Icon name="sparkle" size={18} />Generate plan
		{/if}
	</button>
	{#if busy}<p class="text-base-content/60 mt-2 text-sm">Local models can take a minute or two.</p>{/if}
</section>

<section class="mt-8">
	<h2 class="mb-3 text-xl font-bold">Your plans</h2>
	{#if data.plans.length === 0}
		<p class="text-base-content/60">No plans yet. Generate one above and its shopping list is built for you.</p>
	{:else}
		<ul class="divide-base-content/10 border-base-content/10 divide-y border-t">
			{#each data.plans as p (p.id)}
				<li>
					<a href="/plans/{p.id}" class="group block py-4">
						<p class="font-display text-lg font-bold group-hover:underline">{p.title}</p>
						<p class="text-base-content/60 text-sm">
							{p.days.length} {p.days.length === 1 ? 'day' : 'days'}, {p.mealCount} meals. Made {fmt(p.createdAt)}
						</p>
						{#if p.notes}<p class="mt-1 truncate text-sm italic">“{p.notes}”</p>{/if}
					</a>
				</li>
			{/each}
		</ul>
	{/if}
</section>
