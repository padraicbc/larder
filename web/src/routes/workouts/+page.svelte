<script lang="ts">
	import { untrack } from 'svelte';
	import { invalidateAll } from '$app/navigation';
	import { api, errorText } from '#lib/api.ts';
	import { longDate, toISO, todayISO } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	const today = todayISO();
	let selected = $state(today);
	let month = $state(today.slice(0, 7)); // YYYY-MM

	// ---- calendar grid (weeks start on Monday) ----
	const monthStart = $derived(new Date(Number(month.slice(0, 4)), Number(month.slice(5)) - 1, 1));
	const monthLabel = $derived(monthStart.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }));
	const cells = $derived.by(() => {
		const lead = (monthStart.getDay() + 6) % 7;
		const days = new Date(monthStart.getFullYear(), monthStart.getMonth() + 1, 0).getDate();
		const out: (string | null)[] = Array(lead).fill(null);
		for (let d = 1; d <= days; d++) out.push(toISO(new Date(monthStart.getFullYear(), monthStart.getMonth(), d)));
		while (out.length % 7) out.push(null);
		return out;
	});
	function shift(n: number) {
		month = toISO(new Date(monthStart.getFullYear(), monthStart.getMonth() + n, 1)).slice(0, 7);
	}
	function jumpToday() {
		month = today.slice(0, 7);
		selected = today;
	}

	const byDate = $derived.by(() => {
		const m = new Map<string, typeof data.workouts>();
		for (const w of data.workouts) m.set(w.date, [...(m.get(w.date) ?? []), w]);
		return m;
	});
	const total = (list: { minutes: number }[] = []) => list.reduce((n, w) => n + w.minutes, 0);
	const burned = (list: { calories?: number }[] = []) => list.reduce((n, w) => n + (w.calories ?? 0), 0);
	const fmtMins = (n: number) => (n >= 60 ? `${Math.floor(n / 60)}h${n % 60 ? ` ${n % 60}m` : ''}` : `${n}m`);

	const inMonth = $derived(data.workouts.filter((w) => w.date.startsWith(month)));
	const monthKinds = $derived(
		Object.entries(inMonth.reduce<Record<string, number>>((acc, w) => ({ ...acc, [w.kind]: (acc[w.kind] ?? 0) + w.minutes }), {})).sort((a, b) => b[1] - a[1])
	);
	const activeDays = $derived(new Set(inMonth.map((w) => w.date)).size);

	// ---- add a session ----
	const NEW = '__new__';
	const remembered = (() => {
		try {
			return JSON.parse(localStorage.getItem('larder.workout') ?? '{}') as { kind?: string; minutes?: number };
		} catch {
			return {};
		}
	})();
	let kind = $state(remembered.kind ?? untrack(() => data.kinds[0]));
	let newKind = $state('');
	let minutes = $state<number | null>(remembered.minutes ?? 30);
	let calories = $state<number | null>(null);
	let note = $state('');
	let saving = $state(false);

	const chosenKind = $derived((kind === NEW ? newKind : kind).replace(/\s+/g, ' ').trim());
	const canSave = $derived(!!chosenKind && minutes !== null && minutes >= 1 && minutes <= 1440 && (calories === null || (calories >= 0 && calories <= 20000)) && !saving);
	const custom = $derived(data.kinds.filter((k) => !data.builtIn.includes(k)));

	async function save() {
		if (!canSave) return;
		saving = true;
		try {
			await api('/workouts', { body: { date: selected, kind: chosenKind, minutes, calories: calories ?? 0, note } });
			try {
				localStorage.setItem('larder.workout', JSON.stringify({ kind: chosenKind, minutes }));
			} catch {}
			const wasNew = kind === NEW;
			await invalidateAll();
			if (wasNew) kind = data.kinds.find((k) => k.toLowerCase() === chosenKind.toLowerCase()) ?? chosenKind;
			newKind = '';
			note = '';
			calories = null;
			toast(`${chosenKind} · ${minutes} min saved${wasNew ? '. Added to your list of kinds' : ''}`);
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			saving = false;
		}
	}

	async function remove(id: string) {
		try {
			await api(`/workouts/${id}`, { method: 'DELETE' });
			await invalidateAll();
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function forgetKind(k: string) {
		if (!confirm(`Remove “${k}” from your list? Sessions you already logged keep their label.`)) return;
		try {
			await api(`/workout-kinds/${encodeURIComponent(k)}`, { method: 'DELETE' });
			if (kind === k) kind = data.kinds[0];
			await invalidateAll();
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	const dayWorkouts = $derived(byDate.get(selected) ?? []);
	const WEEK = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
</script>

<svelte:head><title>Workouts · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Workouts</h1>

<div class="mt-6 grid max-w-5xl gap-8 lg:grid-cols-[1fr_20rem]">
	<section aria-label="Workout calendar">
		<div class="flex items-center justify-between">
			<h2 class="text-xl font-bold">{monthLabel}</h2>
			<div class="flex items-center gap-1">
				<button class="btn btn-ghost btn-sm" onclick={jumpToday}>Today</button>
				<button class="btn btn-ghost btn-square btn-sm" aria-label="Previous month" onclick={() => shift(-1)}><Icon name="left" size={18} /></button>
				<button class="btn btn-ghost btn-square btn-sm" aria-label="Next month" onclick={() => shift(1)}><Icon name="right" size={18} /></button>
			</div>
		</div>

		<div class="mt-3 grid grid-cols-7 gap-1 text-center">
			{#each WEEK as d (d)}<span class="text-base-content/60 text-xs font-semibold">{d}</span>{/each}
			{#each cells as iso, i (iso ?? `pad${i}`)}
				{#if iso}
					{@const list = byDate.get(iso)}
					<button
						class="rounded-field flex aspect-square flex-col items-center justify-start gap-0.5 border p-1 text-sm transition-colors
							{selected === iso ? 'border-primary bg-primary/10' : 'border-transparent hover:bg-base-200'}
							{iso === today ? 'font-extrabold' : ''}"
						aria-pressed={selected === iso}
						aria-label="{longDate(iso)}{list ? `: ${list.map((w) => `${w.kind} ${w.minutes} minutes${w.calories ? `, ${w.calories} kilocalories` : ''}`).join(', ')}` : ''}"
						onclick={() => (selected = iso)}
					>
						<span class="tabular {iso === today ? 'text-primary' : ''}">{Number(iso.slice(8))}</span>
						{#if list}
							<span class="bg-primary h-1.5 rounded-full" style="width: {Math.min(60, 12 + total(list) / 3)}%" aria-hidden="true"></span>
							<span class="tabular text-base-content/70 hidden text-[10px] leading-none sm:block">{fmtMins(total(list))}</span>
						{/if}
					</button>
				{:else}
					<span></span>
				{/if}
			{/each}
		</div>

		<div class="mt-4 text-sm">
			{#if inMonth.length}
				<p>
					<strong class="tabular">{inMonth.length}</strong> {inMonth.length === 1 ? 'session' : 'sessions'} on
					<strong class="tabular">{activeDays}</strong> {activeDays === 1 ? 'day' : 'days'},
					<strong class="tabular">{fmtMins(total(inMonth))}</strong> in {monthStart.toLocaleDateString(undefined, { month: 'long' })}{#if burned(inMonth)}, about <strong class="tabular">{burned(inMonth)}</strong> kcal burned{/if}
				</p>
				<p class="text-base-content/70 mt-1">{monthKinds.map(([k, m]) => `${k} ${fmtMins(m)}`).join(' · ')}</p>
			{:else}
				<p class="text-base-content/60">Nothing logged this month yet.</p>
			{/if}
		</div>
	</section>

	<section class="bg-base-200 rounded-box h-fit p-5" aria-label="Selected day">
		<h2 class="text-lg font-bold">{longDate(selected)}</h2>

		{#if dayWorkouts.length}
			<ul class="divide-base-content/10 mt-2 divide-y">
				{#each dayWorkouts as w (w.id)}
					<li class="flex items-center gap-2 py-2">
						<div class="min-w-0 flex-1">
							<p class="font-semibold">{w.kind} <span class="tabular text-base-content/70 font-normal">· {w.minutes} min{#if w.calories} · {w.calories} kcal{/if}</span></p>
							{#if w.note}<p class="text-base-content/60 truncate text-xs">{w.note}</p>{/if}
						</div>
						<button class="btn btn-ghost btn-square btn-xs" aria-label="Delete {w.kind}" onclick={() => remove(w.id)}><Icon name="trash" size={14} /></button>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="text-base-content/60 mt-1 text-sm">No workouts this day.</p>
		{/if}

		<div class="border-base-content/10 mt-4 border-t pt-4">
			<p class="text-sm font-semibold">Add a workout</p>
			<label class="mt-2 block">
				<span class="mb-1 block text-xs">Kind</span>
				<select class="select select-sm w-full" bind:value={kind}>
					{#each data.kinds as k (k)}<option value={k}>{k}</option>{/each}
					<option value={NEW}>＋ Add a new kind…</option>
				</select>
			</label>
			{#if kind === NEW}
				<input class="input input-sm mt-2 w-full" maxlength="40" placeholder="e.g. Padel, Rowing, Climbing" aria-label="New kind of workout" bind:value={newKind} />
				<p class="text-base-content/60 mt-1 text-xs">It’s saved to the list for next time.</p>
			{/if}

			<label class="mt-3 block">
				<span class="mb-1 block text-xs">Duration (minutes)</span>
				<input class="input input-sm tabular w-full" type="number" inputmode="numeric" min="1" max="1440" bind:value={minutes} />
			</label>
			<div class="mt-2 flex flex-wrap gap-1">
				{#each [15, 30, 45, 60, 90] as m (m)}
					<button type="button" class="btn btn-xs rounded-full {minutes === m ? 'btn-primary' : 'btn-outline border-base-300'}" onclick={() => (minutes = m)}>{m}</button>
				{/each}
			</div>

			<label class="mt-3 block">
				<span class="mb-1 block text-xs">Calories burned <span class="text-base-content/50">(optional)</span></span>
				<input class="input input-sm tabular w-full" type="number" inputmode="numeric" min="0" max="20000" placeholder="From your watch or tracker" bind:value={calories} />
			</label>
			<input class="input input-sm mt-3 w-full" maxlength="200" placeholder="Note (optional): 5k, felt easy" aria-label="Note" bind:value={note} />
			<button class="btn btn-primary btn-sm mt-3 w-full" disabled={!canSave} onclick={save}>Add to {selected === today ? 'today' : longDate(selected)}</button>
		</div>

		{#if custom.length}
			<details class="mt-4">
				<summary class="cursor-pointer text-xs font-semibold">Your own kinds ({custom.length})</summary>
				<ul class="mt-2 flex flex-wrap gap-1">
					{#each custom as k (k)}
						<li class="badge badge-outline gap-1">
							{k}<button class="ml-1" aria-label="Remove {k} from my kinds" onclick={() => forgetKind(k)}>✕</button>
						</li>
					{/each}
				</ul>
			</details>
		{/if}
	</section>
</div>
