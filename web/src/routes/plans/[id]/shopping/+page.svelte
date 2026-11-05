<script lang="ts">
	import { untrack } from 'svelte';
	import { api, errorText } from '#lib/api.ts';
	import { cap, fmtQty, groupBy, inPantry, shoppingText } from '#lib/format.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import type { ShoppingItem, ShoppingList } from '#lib/types.ts';
	import Icon from '#lib/components/Icon.svelte';

	let { data } = $props();

	// Local copy so ticking feels instant; the server response replaces it.
	let items = $state<ShoppingItem[]>(untrack(() => data.list.items));
	$effect(() => {
		items = data.list.items;
	});

	let hideDone = $state(false);
	let newItem = $state('');
	const base = $derived(`/plans/${data.plan.id}/shopping`);
	const done = $derived(items.filter((i) => i.checked).length);
	const have = $derived(new Set(items.filter((i) => inPantry(i.name, data.pantryNames)).map((i) => i.key)));
	const groups = $derived(groupBy(items.filter((i) => !(hideDone && i.checked)), (i) => i.category));

	async function toggle(item: ShoppingItem) {
		const checked = !item.checked;
		item.checked = checked;
		try {
			const l = await api<ShoppingList>(`${base}/items/${encodeURIComponent(item.key)}`, {
				method: 'PATCH',
				body: { checked }
			});
			items = l.items;
		} catch (e) {
			item.checked = !checked;
			toast(errorText(e), 'error');
		}
	}

	async function add() {
		const name = newItem.trim();
		if (!name) return;
		try {
			const l = await api<ShoppingList>(`${base}/items`, { body: { name } });
			items = l.items;
			newItem = '';
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function rebuild() {
		try {
			const l = await api<ShoppingList>(`${base}/rebuild`, { method: 'POST' });
			items = l.items;
			toast('Quantities recalculated');
		} catch (e) {
			toast(errorText(e), 'error');
		}
	}

	async function copy() {
		try {
			await navigator.clipboard.writeText(shoppingText(items));
			toast('List copied');
		} catch {
			toast('Your browser blocked the clipboard', 'error');
		}
	}
</script>

<svelte:head><title>Shopping list · Larder</title></svelte:head>

<a href="/plans/{data.plan.id}" class="btn btn-ghost btn-sm -ml-2"><Icon name="left" size={16} />{data.plan.title}</a>

<div class="mt-2 flex flex-wrap items-end justify-between gap-3">
	<div>
		<h1 class="text-3xl font-extrabold lg:text-4xl">Shopping list</h1>
		<p class="text-base-content/70 tabular mt-1">
			{done} of {items.length} in the basket{#if have.size}. {have.size} might already be in your <a class="link" href="/pantry">pantry</a>{/if}
		</p>
	</div>
	<div class="flex flex-wrap gap-2">
		<button class="btn btn-sm" onclick={copy}><Icon name="copy" size={16} />Copy list</button>
		<button class="btn btn-sm btn-ghost" onclick={rebuild} title="Use after changing household size">
			<Icon name="refresh" size={16} />Recalculate
		</button>
	</div>
</div>

<progress class="progress progress-primary mt-4 w-full" value={done} max={items.length || 1}></progress>

<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
	<div class="join w-full sm:w-auto">
		<input
			class="input join-item w-full sm:w-72"
			placeholder="Add something else"
			bind:value={newItem}
			onkeydown={(e) => e.key === 'Enter' && add()}
		/>
		<button class="btn btn-primary join-item" onclick={add} disabled={!newItem.trim()}>Add</button>
	</div>
	<label class="flex cursor-pointer items-center gap-2 text-sm">
		<input type="checkbox" class="toggle toggle-sm" bind:checked={hideDone} />Hide ticked items
	</label>
</div>

<div class="mt-6 grid gap-6 sm:grid-cols-2 xl:grid-cols-3">
	{#each groups as [category, list] (category)}
		<section>
			<h2 class="mb-2 text-lg font-bold">{cap(category)}</h2>
			<ul class="bg-base-200 rounded-box p-1">
				{#each list as item (item.key)}
					<li>
						<label class="hover:bg-base-300 rounded-field flex cursor-pointer items-center gap-3 px-3 py-2.5">
							<input type="checkbox" class="checkbox checkbox-primary" checked={item.checked} onchange={() => toggle(item)} />
							<span class="flex-1 {item.checked ? 'text-base-content/40 line-through' : ''}">{cap(item.name)}</span>
							{#if have.has(item.key) && !item.checked}<span class="badge badge-sm badge-success badge-soft">in pantry</span>{/if}
							{#if item.quantity}
								<span class="tabular text-base-content/60 text-sm">{fmtQty(item.quantity, item.unit)}</span>
							{/if}
						</label>
					</li>
				{/each}
			</ul>
		</section>
	{/each}
</div>
