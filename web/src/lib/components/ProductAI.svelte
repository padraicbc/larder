<script lang="ts">
 import { api, errorText } from '#lib/api.ts';
 import type { Scan } from '#lib/types.ts';
 let { name = '', onaccept }: { name?: string; onaccept: (suggestion: Scan) => void | Promise<void> } = $props();
 let description = $state('');
 let busy = $state(false);
 let error = $state('');
 let suggestion = $state<Scan | null>(null);
 const fields = [['calories','Calories'],['protein','Protein (g)'],['carbs','Carbs (g)'],['fat','Fat (g)'],['saturatedFat','Saturated fat (g, if known)'],['fiber','Fibre (g)']] as const;
 const valid = $derived(!!suggestion && !!suggestion.name.trim() && (!suggestion.hasNutrition || (suggestion.basisQuantity === 100 && (suggestion.basisUnit === 'g' || suggestion.basisUnit === 'ml') && fields.every(([key]) => key === 'saturatedFat' && suggestion!.macros[key] == null || typeof suggestion!.macros[key] === 'number' && Number.isFinite(suggestion!.macros[key]) && suggestion!.macros[key]! >= 0) && (suggestion.macros.saturatedFat == null || suggestion.macros.saturatedFat <= suggestion.macros.fat))) && (suggestion.hasNutrition || !!suggestion.ingredients.trim()));
 async function ask() {
  busy = true; error = ''; suggestion = null;
  try { suggestion = await api<Scan>('/products/suggest', { body: { description: description.trim() || name } }); }
  catch (e) { error = errorText(e); }
  finally { busy = false; }
 }
 async function accept() {
  if (!suggestion || !valid) return;
  busy = true; error = '';
  try { await onaccept(suggestion); suggestion = null; }
  catch (e) { error = errorText(e); }
  finally { busy = false; }
 }
</script>

<details class="rounded-box border-base-300 mt-3 border p-3">
 <summary class="cursor-pointer text-sm font-semibold">No package or database match? Ask AI</summary>
 <p class="text-base-content/70 mt-2 text-sm">Describe the food, brand, variety and country if known. AI can suggest nutrition and ingredients from model knowledge; this does not search the web. Estimates may differ from the actual recipe.</p>
 <label class="mt-3 block text-sm">Food description<input class="input mt-1 w-full" maxlength="1000" placeholder={name || 'e.g. full-fat vintage cheddar, Ireland'} bind:value={description} disabled={busy} /></label>
 <button class="btn btn-sm mt-2" disabled={busy || !(description.trim() || name.trim())} onclick={ask}>{#if busy}<span class="loading loading-spinner loading-sm"></span>{/if}Ask AI</button>
 {#if error}<p role="alert" class="text-error mt-2 text-sm">{error}</p>{/if}
 {#if suggestion}
  <div class="bg-base-200 rounded-box mt-3 p-3">
   <p class="font-semibold">Review AI estimate</p>
   <p class="text-base-content/70 mt-1 text-sm">{suggestion.notes} A suggested ingredient list cannot confirm allergens.</p>
   <label class="mt-2 block text-xs">Name<input class="input input-sm mt-1 w-full" bind:value={suggestion.name} /></label>
   <label class="mt-2 block text-xs">Suggested ingredients<textarea class="textarea mt-1 w-full" rows="3" bind:value={suggestion.ingredients} placeholder="Not known — leave empty"></textarea></label>
   {#if suggestion.hasNutrition}
    <p class="mt-2 text-sm font-semibold">Nutrition per 100 {suggestion.basisUnit}</p>
    <div class="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3">
     {#each fields as [key,label] (key)}<label class="text-xs">{label}<input class="input input-sm mt-1 w-full" type="number" min="0" step="any" inputmode="decimal" bind:value={suggestion.macros[key]} /></label>{/each}
    </div>
   {:else}<p class="mt-2 text-sm">The model could not estimate nutrition for this food.</p>{/if}
   <p class="text-base-content/70 mt-2 text-xs">Existing ingredients and nutrition are kept; this fills missing information only. Save afterwards to keep the changes.</p>
   <div class="mt-3 flex gap-2"><button class="btn btn-primary btn-sm" disabled={busy || !valid} onclick={accept}>Use estimate</button><button class="btn btn-ghost btn-sm" disabled={busy} onclick={() => suggestion = null}>Discard</button></div>
  </div>
 {/if}
</details>
