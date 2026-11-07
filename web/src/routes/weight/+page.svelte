<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { api, errorText } from '#lib/api.ts';
	import { longDate, todayISO } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { RANGES, dateTicks, dayNumber, inRange, niceTicks, rangeStart, summarize, type Range } from '#lib/weight-chart.ts';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	const today = todayISO();
	let range = $state<Range>(
		(() => {
			try {
				const saved = localStorage.getItem('larder.weightRange');
				return RANGES.some((r) => r.id === saved) ? (saved as Range) : '6m';
			} catch {
				return '6m';
			}
		})()
	);
	function pick(r: Range) {
		range = r;
		try {
			localStorage.setItem('larder.weightRange', r);
		} catch {}
	}

	// ---- entry ----
	let date = $state(today);
	let kg = $state<number | null>(null);
	let saving = $state(false);
	const existing = $derived(data.entries.find((e) => e.date === date));
	const canSave = $derived(kg !== null && kg >= 20 && kg <= 500 && date <= today && !saving);

	async function save() {
		if (!canSave) return;
		saving = true;
		try {
			await api('/weight', { method: 'PUT', body: { date, kg } });
			await invalidateAll();
			toast(existing ? 'Weight updated' : 'Weight saved');
			kg = null;
		} catch (e) {
			toast(errorText(e), 'error');
		} finally {
			saving = false;
		}
	}

	async function remove(d: string) {
		try {
			await api(`/weight/${d}`, { method: 'DELETE' });
			await invalidateAll();
			toast('Entry removed');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	// ---- chart ----
	const start = $derived(rangeStart(range, today, data.entries));
	const visible = $derived(inRange(data.entries, start, today));
	const stats = $derived(summarize(visible));

	const H = 280;
	const M = { top: 12, right: 16, bottom: 28, left: 44 };
	let W = $state(640);

	const x0 = $derived(dayNumber(start));
	const x1 = $derived(Math.max(dayNumber(today), x0 + 1));
	const yTicks = $derived(
		stats ? niceTicks(stats.min - 0.5, stats.max + 0.5) : [0, 1]
	);
	const yMin = $derived(yTicks[0]);
	const yMax = $derived(yTicks[yTicks.length - 1]);
	const px = (d: string) => M.left + ((dayNumber(d) - x0) / (x1 - x0)) * (W - M.left - M.right);
	const py = (v: number) => M.top + (1 - (v - yMin) / (yMax - yMin || 1)) * (H - M.top - M.bottom);
	const points = $derived(visible.map((e) => ({ ...e, x: px(e.date), y: py(e.kg) })));
	const line = $derived(points.map((p, i) => `${i ? 'L' : 'M'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(''));
	const xTicks = $derived(dateTicks(start, today, W < 480 ? 3 : 5));
	const longRange = $derived(x1 - x0 > 300);
	const tickLabel = (iso: string) => {
		const [y, m, d] = iso.split('-').map(Number);
		return new Date(y, m - 1, d).toLocaleDateString(undefined, longRange ? { month: 'short', year: '2-digit' } : { day: 'numeric', month: 'short' });
	};

	// ---- hover / keyboard: snap to the nearest reading ----
	let active = $state<number | null>(null);
	const shown = $derived(active !== null ? points[active] : null);
	function nearest(clientX: number, svg: SVGSVGElement) {
		const x = clientX - svg.getBoundingClientRect().left;
		let best = 0;
		points.forEach((p, i) => {
			if (Math.abs(p.x - x) < Math.abs(points[best].x - x)) best = i;
		});
		active = points.length ? best : null;
	}
	function key(e: KeyboardEvent) {
		if (!points.length) return;
		const step = e.key === 'ArrowLeft' ? -1 : e.key === 'ArrowRight' ? 1 : 0;
		if (!step) return;
		e.preventDefault();
		active = Math.min(Math.max((active ?? (step > 0 ? -1 : points.length)) + step, 0), points.length - 1);
	}
	const showDots = $derived(points.length <= 45);
	const fmtChange = (n: number) => `${n > 0 ? '+' : ''}${n.toFixed(1)} kg`;
	const rangeLabel = $derived(RANGES.find((r) => r.id === range)?.label.toLowerCase() ?? '');
</script>

<svelte:head><title>Weight · Larder</title></svelte:head>

<h1 class="text-3xl font-extrabold lg:text-4xl">Weight</h1>

<section class="bg-base-200 rounded-box mt-6 max-w-3xl p-5">
	<h2 class="text-lg font-bold">Log your weight</h2>
	<div class="mt-3 flex flex-wrap items-end gap-3">
		<label>
			<span class="mb-1 block text-sm font-semibold">Date</span>
			<input class="input" type="date" max={today} bind:value={date} />
		</label>
		<label>
			<span class="mb-1 block text-sm font-semibold">Weight (kg)</span>
			<input
				class="input tabular w-32"
				type="number"
				inputmode="decimal"
				min="20"
				max="500"
				step="0.1"
				placeholder={existing ? String(existing.kg) : data.entries.at(-1) ? String(data.entries.at(-1)?.kg) : 'e.g. 82.4'}
				bind:value={kg}
				onkeydown={(e) => e.key === 'Enter' && save()}
			/>
		</label>
		<button class="btn btn-primary" disabled={!canSave} onclick={save}>{existing ? 'Update' : 'Save'}</button>
	</div>
	{#if existing}<p class="text-base-content/60 mt-2 text-xs">You already logged {existing.kg} kg for this day; saving replaces it.</p>{/if}
</section>

<!-- One filter row above the chart; it scopes the numbers, chart and table below. -->
<div class="mt-8 flex max-w-3xl flex-wrap gap-2" role="group" aria-label="Time range">
	{#each RANGES as r (r.id)}
		<button
			type="button"
			class="btn btn-sm rounded-full {range === r.id ? 'btn-primary' : 'btn-outline border-base-300'}"
			aria-pressed={range === r.id}
			onclick={() => pick(r.id)}>{r.label}</button
		>
	{/each}
</div>

{#if data.entries.length === 0}
	<p class="text-base-content/60 mt-8 max-w-3xl text-sm">No readings yet. Log your weight above and your trend will show up here.</p>
{:else}
	<div class="mt-5 grid max-w-3xl grid-cols-3 gap-3">
		<div>
			<p class="text-base-content/60 text-xs">Latest</p>
			<p class="font-display tabular text-2xl font-bold">{data.entries.at(-1)?.kg} <span class="text-base-content/60 text-sm font-normal">kg</span></p>
			<p class="text-base-content/50 text-xs">{longDate(data.entries.at(-1)?.date ?? today)}</p>
		</div>
		<div>
			<p class="text-base-content/60 text-xs">Change, {rangeLabel}</p>
			<p class="font-display tabular text-2xl font-bold">{stats && stats.first !== stats.last ? fmtChange(stats.change) : '–'}</p>
			<p class="text-base-content/50 text-xs">{stats && stats.first !== stats.last ? `since ${longDate(stats.first.date)}` : 'needs two readings'}</p>
		</div>
		<div>
			<p class="text-base-content/60 text-xs">Range</p>
			<p class="font-display tabular text-2xl font-bold">{stats ? `${stats.min}–${stats.max}` : '–'}</p>
			<p class="text-base-content/50 text-xs">lowest to highest</p>
		</div>
	</div>

	<div class="relative mt-4 max-w-3xl" bind:clientWidth={W}>
		{#if visible.length === 0}
			<p class="text-base-content/60 py-16 text-center text-sm">No readings in the last {rangeLabel}. Try a longer range.</p>
		{:else}
			<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
			<svg
				width={W}
				height={H}
				role="img"
				tabindex="0"
				aria-label="Weight over the last {rangeLabel}: {visible.length} readings from {stats?.first.kg} to {stats?.last.kg} kg. Use the left and right arrow keys to step through readings."
				class="touch-pan-y select-none"
				onpointermove={(e) => nearest(e.clientX, e.currentTarget)}
				onpointerdown={(e) => nearest(e.clientX, e.currentTarget)}
				onpointerleave={() => (active = null)}
				onkeydown={key}
				onblur={() => (active = null)}
			>
				<!-- recessive grid + axes -->
				{#each yTicks as t (t)}
					<line x1={M.left} x2={W - M.right} y1={py(t)} y2={py(t)} stroke="currentColor" stroke-opacity="0.12" />
					<text x={M.left - 8} y={py(t)} dy="0.32em" text-anchor="end" font-size="11" fill="currentColor" fill-opacity="0.6" class="tabular">{t}</text>
				{/each}
				{#each xTicks as t, i (t)}
					<text x={px(t)} y={H - 8} text-anchor={i === 0 ? 'start' : i === xTicks.length - 1 ? 'end' : 'middle'} font-size="11" fill="currentColor" fill-opacity="0.6">{tickLabel(t)}</text>
				{/each}

				<path d={line} fill="none" stroke="var(--color-primary)" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
				{#if showDots}
					{#each points as p (p.date)}
						<circle cx={p.x} cy={p.y} r="4" fill="var(--color-primary)" stroke="var(--color-base-100)" stroke-width="2" />
					{/each}
				{:else if points.length}
					<circle cx={points.at(-1)?.x} cy={points.at(-1)?.y} r="4" fill="var(--color-primary)" stroke="var(--color-base-100)" stroke-width="2" />
				{/if}

				{#if shown}
					<line x1={shown.x} x2={shown.x} y1={M.top} y2={H - M.bottom} stroke="currentColor" stroke-opacity="0.35" stroke-dasharray="3 3" />
					<circle cx={shown.x} cy={shown.y} r="6" fill="var(--color-primary)" stroke="var(--color-base-100)" stroke-width="2" />
				{/if}
			</svg>
			{#if shown}
				<div
					class="bg-base-100 border-base-300 rounded-box pointer-events-none absolute z-10 border px-3 py-2 text-sm shadow-md"
					style="top: {Math.max(shown.y - 56, 0)}px; left: {Math.min(Math.max(shown.x - 60, 0), W - 130)}px"
					role="status"
				>
					<p class="tabular font-bold">{shown.kg} kg</p>
					<p class="text-base-content/60 text-xs">{longDate(shown.date)}</p>
				</div>
			{/if}
		{/if}
	</div>

	<details class="mt-6 max-w-3xl">
		<summary class="cursor-pointer text-sm font-semibold">Show readings as a table ({visible.length})</summary>
		<table class="table table-sm mt-2">
			<thead><tr><th>Date</th><th class="text-right">kg</th><th class="text-right">vs previous</th><th></th></tr></thead>
			<tbody>
				{#each visible.toReversed() as e, i (e.date)}
					{@const prev = visible[visible.length - 2 - i]}
					<tr>
						<td>{longDate(e.date)}</td>
						<td class="tabular text-right font-semibold">{e.kg}</td>
						<td class="tabular text-base-content/60 text-right">{prev ? fmtChange(Math.round((e.kg - prev.kg) * 10) / 10) : '–'}</td>
						<td class="text-right"><button class="btn btn-ghost btn-xs btn-square" aria-label="Delete {e.date}" onclick={() => remove(e.date)}><Icon name="trash" size={14} /></button></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</details>
{/if}
