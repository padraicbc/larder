<script lang="ts">
	let {
		tags = $bindable([]),
		placeholder = 'Type and press Enter',
		suggestions = [],
		tone = 'neutral',
		id
	}: { tags?: string[]; placeholder?: string; suggestions?: string[]; tone?: 'neutral' | 'error'; id?: string } =
		$props();

	let draft = $state('');
	const unused = $derived(suggestions.filter((s) => !tags.some((t) => t.toLowerCase() === s.toLowerCase())));

	function add(v: string) {
		const t = v.trim();
		if (t && !tags.some((x) => x.toLowerCase() === t.toLowerCase())) tags = [...tags, t];
		draft = '';
	}
	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ',') {
			e.preventDefault();
			add(draft);
		} else if (e.key === 'Backspace' && !draft && tags.length) {
			tags = tags.slice(0, -1);
		}
	}
</script>

<div
	class="input h-auto min-h-10 w-full flex-wrap gap-1.5 py-1.5 {tone === 'error' ? 'input-error' : ''}"
>
	{#each tags as tag (tag)}
		<span class="badge {tone === 'error' ? 'badge-error badge-soft' : 'badge-neutral badge-soft'} gap-1">
			{tag}
			<button
				type="button"
				class="cursor-pointer opacity-60 hover:opacity-100"
				aria-label="Remove {tag}"
				onclick={() => (tags = tags.filter((t) => t !== tag))}>✕</button
			>
		</span>
	{/each}
	<input {id} class="min-w-28 flex-1 bg-transparent outline-none" {placeholder} bind:value={draft} {onkeydown} onblur={() => add(draft)} />
</div>
{#if unused.length}
	<div class="mt-2 flex flex-wrap gap-1.5">
		{#each unused as s (s)}
			<button type="button" class="btn btn-xs btn-ghost border-base-300 border border-dashed" onclick={() => add(s)}>+ {s}</button>
		{/each}
	</div>
{/if}
