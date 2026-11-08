<script lang="ts">
	import type { ProductRating } from '#lib/types.ts';

	let {
		rating,
		name = '',
		variant = 'badge'
	}: { rating: ProductRating; name?: string; variant?: 'badge' | 'link' } = $props();

	let dialog: HTMLDialogElement;

	const BADGE: Record<ProductRating['label'], string> = {
		excellent: 'badge-success',
		good: 'badge-success badge-soft',
		fair: 'badge-warning',
		poor: 'badge-error badge-soft',
		avoid: 'badge-error'
	};
	const BANDS = [
		['excellent', '85–100', 'Simple ingredients, few or no concerns.'],
		['good', '70–84', 'Mostly fine, with a small concern or two.'],
		['fair', '50–69', 'Several concerns; fine now and then.'],
		['poor', '30–49', 'Heavily processed or high in sugar/additives.'],
		['avoid', '0–29', 'Many concerns at once.']
	] as const;

	const lowers = $derived(rating.factors.filter((f) => f.points < 0));
	const raises = $derived(rating.factors.filter((f) => f.points > 0));
	const notes = $derived(rating.factors.filter((f) => f.points === 0));
	const raw = $derived(rating.base + rating.factors.reduce((n, f) => n + f.points, 0));
	const sum = (list: { points: number }[]) => list.reduce((n, f) => n + f.points, 0);
	const sign = (n: number) => (n > 0 ? `+${n}` : `${n}`);

	const headline = $derived(
		rating.label === 'excellent' || rating.label === 'good'
			? lowers.length
				? `Rated ${rating.label} overall, with ${lowers.length === 1 ? 'one thing' : `${lowers.length} things`} marking it down.`
				: `Rated ${rating.label}: nothing in the ingredients or nutrition marks it down.`
			: `Rated ${rating.label} mainly because of ${lowers
					.slice(0, 2)
					.map((f) => f.text.replace(/^E\d+\w*: /, '').toLowerCase())
					.join(' and ')}.`
	);
</script>

{#if variant === 'badge'}
	<button type="button" class="badge {BADGE[rating.label]} cursor-pointer shrink-0" aria-haspopup="dialog" aria-label="{rating.score}% {rating.label}. Why this rating?" onclick={() => dialog.showModal()}>
		{rating.score}% <span class="ml-1 opacity-70" aria-hidden="true">ⓘ</span>
	</button>
{:else}
	<button type="button" class="link link-primary text-sm" aria-haspopup="dialog" onclick={() => dialog.showModal()}>Why {rating.score}%?</button>
{/if}

<dialog bind:this={dialog} class="modal modal-bottom sm:modal-middle">
	<div class="modal-box max-h-[85dvh]">
		<div class="flex items-start justify-between gap-3">
			<div>
				{#if name}<p class="text-base-content/60 text-sm">{name}</p>{/if}
				<h3 class="text-2xl font-bold capitalize">{rating.label} · <span class="tabular">{rating.score}%</span></h3>
			</div>
			<form method="dialog"><button class="btn btn-ghost btn-sm btn-square" aria-label="Close">✕</button></form>
		</div>

		{#if rating.provisional}
			<p class="alert alert-warning mt-3 text-sm font-semibold" role="note">
				Provisional: {rating.provisionalReason || "Some ingredients or nutrition data could not be fully checked."}
			</p>
		{/if}

		<progress class="progress {rating.score >= 70 ? 'progress-success' : rating.score >= 50 ? 'progress-warning' : 'progress-error'} mt-3 w-full" value={rating.score} max="100" aria-hidden="true"></progress>
		<p class="mt-3 text-sm">{headline}</p>

		<p class="text-base-content/70 mt-3 text-sm">
			How it’s worked out: every product starts at <strong class="tabular">{rating.base}</strong>. Things that are worth knowing about take points off; good things add some back.
		</p>

		{#if lowers.length}
			<h4 class="mt-5 text-sm font-bold">What lowers it <span class="tabular text-error">{sign(sum(lowers))}</span></h4>
			<ul class="mt-2 space-y-3">
				{#each lowers as f, i (i)}
					<li>
						<p class="flex justify-between gap-3 text-sm font-semibold"><span>{f.text}</span><span class="tabular text-error">{sign(f.points)}</span></p>
						{#if f.why}<p class="text-base-content/70 mt-0.5 text-sm">{f.why}</p>{/if}
					</li>
				{/each}
			</ul>
		{/if}

		{#if raises.length}
			<h4 class="mt-5 text-sm font-bold">What raises it <span class="tabular text-success">{sign(sum(raises))}</span></h4>
			<ul class="mt-2 space-y-3">
				{#each raises as f, i (i)}
					<li>
						<p class="flex justify-between gap-3 text-sm font-semibold"><span>{f.text}</span><span class="tabular text-success">{sign(f.points)}</span></p>
						{#if f.why}<p class="text-base-content/70 mt-0.5 text-sm">{f.why}</p>{/if}
					</li>
				{/each}
			</ul>
		{/if}

		{#each notes as f, i (i)}
			<p class="bg-base-200 rounded-field mt-4 p-3 text-sm"><strong>{f.text}.</strong> {f.why}</p>
		{/each}

		{#if !lowers.length && !raises.length && !notes.length}
			<p class="text-base-content/70 mt-5 text-sm">Nothing stood out in either direction.</p>
		{/if}

		<p class="mt-5 text-sm font-semibold">
			{rating.base}{lowers.length ? ` ${sign(sum(lowers))}` : ''}{raises.length ? ` ${sign(sum(raises))}` : ''} = <span class="tabular">{raw}</span>{raw !== rating.score ? `, kept within 0–100 as ${rating.score}` : ''}
		</p>

		<details class="mt-4">
			<summary class="cursor-pointer text-sm font-semibold">What the ratings mean</summary>
			<ul class="mt-2 space-y-1 text-sm">
				{#each BANDS as [label, range, text] (label)}
					<li class:font-bold={label === rating.label}><span class="capitalize">{label}</span> <span class="tabular text-base-content/60">({range})</span>: {text}</li>
				{/each}
			</ul>
		</details>

		<p class="text-base-content/50 mt-4 text-xs">
			Calculated at {rating.at ? new Date(rating.at).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : 'an unknown time'} from the saved ingredients, with rating rules v{rating.rules ?? 1}. Ratings are never stored: they're worked out again every time you look, so this time changes on each visit.
		</p>
		<p class="text-base-content/60 mt-2 text-xs">
			This is a rough guide from the ingredients list and the macros only. It can’t see salt or total sugar, or how much of each additive is in there, and it isn’t medical advice.
		</p>
		<div class="modal-action"><form method="dialog"><button class="btn">Got it</button></form></div>
	</div>
	<form method="dialog" class="modal-backdrop"><button aria-label="Close">close</button></form>
</dialog>
