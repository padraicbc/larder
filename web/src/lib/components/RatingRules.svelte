<script lang="ts">
	import { onMount } from 'svelte';
	import { api, errorText } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';

	type Rule = { key: string; group: string; label: string; hint?: string; default: number };
	type Custom = { match: string; points: number };
	type View = { settings: { points?: Record<string, number>; custom?: Custom[] }; catalogue: Rule[]; rules: number };

	let catalogue = $state<Rule[]>([]);
	let saved = $state<View['settings']>({});
	let values = $state<Record<string, number | null>>({}); // null/missing = the default
	let custom = $state<Custom[]>([]);
	let rulesVersion = $state(0);
	let loaded = $state(false);
	let saving = $state(false);
	let search = $state('');
	let newMatch = $state('');
	let newPoints = $state<number | null>(-5);

	function load(v: View) {
		catalogue = v.catalogue;
		saved = v.settings;
		rulesVersion = v.rules;
		values = { ...(v.settings.points ?? {}) };
		custom = (v.settings.custom ?? []).map((c) => ({ ...c }));
		loaded = true;
	}
	onMount(async () => {
		try {
			load(await api<View>('/rating-settings'));
		} catch (e) {
			toast(errorText(e), 'error');
		}
	});

	const GROUPS = $derived([...new Set(catalogue.map((r) => r.group))]);
	const effective = (r: Rule) => values[r.key] ?? r.default;
	const changed = (r: Rule) => values[r.key] !== undefined && values[r.key] !== null && values[r.key] !== r.default;
	const changedCount = $derived(catalogue.filter(changed).length + custom.length);

	const current = $derived({
		points: Object.fromEntries(Object.entries(values).filter(([k, v]) => v !== null && v !== undefined && catalogue.find((r) => r.key === k)?.default !== v)),
		custom
	});
	const dirty = $derived(JSON.stringify(current) !== JSON.stringify({ points: saved.points ?? {}, custom: saved.custom ?? [] }));

	function setValue(r: Rule, v: number | null) {
		if (v === null || Number.isNaN(v)) delete values[r.key];
		else values[r.key] = Math.round(v);
	}

	function addCustom() {
		const m = newMatch.replace(/\s+/g, ' ').trim();
		if (m.length < 2 || !newPoints) return;
		custom = [...custom.filter((c) => c.match.toLowerCase() !== m.toLowerCase()), { match: m, points: Math.round(newPoints) }];
		newMatch = '';
	}

	async function save(body: unknown = current) {
		saving = true;
		try {
			load(await api<View>('/rating-settings', { method: 'PUT', body }));
			toast('Rating rules saved. Every rating now follows them.');
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			saving = false;
		}
	}

	async function resetAll() {
		if (!confirm('Put every rating rule back to its default and remove your own rules?')) return;
		await save({ points: {}, custom: [] });
	}

	const q = $derived(search.trim().toLowerCase());
	const rows = (group: string) =>
		catalogue.filter((r) => r.group === group && (group !== 'Specific additives' || !q || r.label.toLowerCase().includes(q) || changed(r)));
	const sign = (n: number) => (n > 0 ? `+${n}` : `${n}`);
</script>

<section class="mt-12 max-w-3xl" aria-label="Rating rules">
	<h2 class="text-xl font-bold">Rating rules</h2>
	<p class="text-base-content/70 mt-1 text-sm">
		How food is scored. These are the defaults: points added or taken off each product's score (it starts at 90). Change any number to make something matter more or less to you, or add your own words. This is only for your account, and ratings follow your changes straight away.
	</p>

	{#if !loaded}
		<p class="text-base-content/60 mt-3 text-sm"><span class="loading loading-spinner loading-sm"></span> Loading…</p>
	{:else}
		<div class="mt-3 flex flex-wrap items-center gap-2">
			<span class="badge {changedCount ? 'badge-primary' : 'badge-ghost'}">{changedCount ? `${changedCount} changed from the defaults` : 'Using the defaults'}</span>
			<span class="text-base-content/50 text-xs">rating rules v{rulesVersion}</span>
		</div>

		<div class="mt-4 flex flex-col gap-3">
			{#each GROUPS as group (group)}
				<details class="border-base-300 rounded-box border" open={group === 'Starting score' || group === 'Ingredient list' || group === 'Number of additives'}>
					<summary class="cursor-pointer p-3 font-semibold">{group}
						{#if catalogue.filter((r) => r.group === group && changed(r)).length}<span class="badge badge-primary badge-sm ml-2">{catalogue.filter((r) => r.group === group && changed(r)).length} changed</span>{/if}
					</summary>
					<div class="px-3 pb-3">
						{#if group === 'Specific additives'}
							<input class="input input-sm mb-2 w-full" type="search" placeholder="Search additives, e.g. palm, E150, sweetener…" aria-label="Search additives" bind:value={search} />
							<p class="text-base-content/60 mb-2 text-xs">Harshest first. Set a normally free additive to a negative number, or a penalised one to 0 to ignore it.</p>
						{/if}
						<ul class="divide-base-content/10 divide-y">
							{#each rows(group) as r (r.key)}
								<li class="flex items-center gap-3 py-2">
									<div class="min-w-0 flex-1">
										<p class="text-sm {changed(r) ? 'font-bold' : ''}">{r.label}</p>
										{#if r.hint}<p class="text-base-content/50 text-xs">{r.hint}</p>{/if}
									</div>
									<span class="text-base-content/50 tabular w-14 text-right text-xs" title="Default">default {sign(r.default)}</span>
									<input
										class="input input-sm tabular w-20 {changed(r) ? 'input-primary' : ''}"
										type="number"
										step="1"
										inputmode="numeric"
										aria-label="Points for {r.label}"
										placeholder={sign(r.default)}
										value={values[r.key] ?? ''}
										oninput={(e) => setValue(r, e.currentTarget.value === '' ? null : e.currentTarget.valueAsNumber)}
									/>
									<button class="btn btn-ghost btn-xs btn-square {changed(r) ? '' : 'invisible'}" aria-label="Back to default for {r.label}" onclick={() => setValue(r, null)}>↺</button>
								</li>
							{:else}
								<li class="text-base-content/60 py-2 text-sm">No additives match.</li>
							{/each}
						</ul>
					</div>
				</details>
			{/each}

			<details class="border-base-300 rounded-box border" open={custom.length > 0}>
				<summary class="cursor-pointer p-3 font-semibold">Your own rules
					{#if custom.length}<span class="badge badge-primary badge-sm ml-2">{custom.length}</span>{/if}
				</summary>
				<div class="px-3 pb-3">
					<p class="text-base-content/60 mb-2 text-xs">Add or take off points whenever a word or phrase is in the ingredients, for example “palm oil” −10, “wholegrain” +4, “aspartame” −15. Whole words only, any capitals.</p>
					<ul class="divide-base-content/10 divide-y">
						{#each custom as c (c.match)}
							<li class="flex items-center gap-3 py-2">
								<span class="min-w-0 flex-1 truncate text-sm font-semibold">“{c.match}”</span>
								<input class="input input-sm tabular w-20" type="number" step="1" aria-label="Points for {c.match}" bind:value={c.points} />
								<button class="btn btn-ghost btn-xs btn-square" aria-label="Remove {c.match}" onclick={() => (custom = custom.filter((x) => x !== c))}>✕</button>
							</li>
						{/each}
					</ul>
					<div class="mt-2 flex gap-2">
						<input class="input input-sm w-full" maxlength="60" placeholder="Word or phrase" aria-label="Word or phrase" bind:value={newMatch} onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addCustom())} />
						<input class="input input-sm tabular w-20" type="number" step="1" aria-label="Points" bind:value={newPoints} />
						<button class="btn btn-sm" disabled={newMatch.trim().length < 2 || !newPoints} onclick={addCustom}>Add</button>
					</div>
				</div>
			</details>
		</div>

		<div class="mt-4 flex flex-wrap gap-2">
			<button class="btn btn-primary" disabled={!dirty || saving} onclick={() => save()}>
				{#if saving}<span class="loading loading-spinner loading-sm"></span>{/if}Save rating rules
			</button>
			{#if dirty}<button class="btn btn-ghost" disabled={saving} onclick={() => load({ settings: saved, catalogue, rules: rulesVersion })}>Discard changes</button>{/if}
			<button class="btn btn-ghost ml-auto" disabled={saving || (!saved.points || !Object.keys(saved.points).length) && !(saved.custom ?? []).length} onclick={resetAll}>Reset everything to defaults</button>
		</div>
	{/if}
</section>
