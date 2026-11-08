<script lang="ts">
 import { onMount } from 'svelte';
 import { invalidateAll } from '$app/navigation';
 import { api, errorText } from '#lib/api.ts';
 import { emptyMacros, MEAL_TYPES, type Product } from '#lib/types.ts';
 import { citationParts, conversationPayload, draftCanSave, safeCitationURL, type FoodChatTurn, type FoodChatSeed, type FoodDraft, type ChatCitation } from '#lib/food-chat.ts';
 let { data } = $props();
 let turns = $state<FoodChatTurn[]>([]);
 let input = $state('');
 let search = $state(false);
 let busy = $state(false);
 let error = $state('');
 let draft = $state<FoodDraft | null>(null);
 let product = $state<Product | null>(null);
 let saved = $state('');
 let barcode = $state('');
 let ready = $state(false);
 let saveError = $state('');
 let itemSnapshot = $state('');
 const itemsDirty = $derived(!!draft && draft.kind === 'meal' && JSON.stringify(draft.items) !== itemSnapshot);
 const storageKey = () => `food-chat:${data.user?.id ?? 'local'}`;
 const fields = [['calories','Calories'],['protein','Protein (g)'],['carbs','Carbs (g)'],['fat','Fat (g)'],['saturatedFat','Saturated fat (g, if known)'],['fiber','Fibre (g)']] as const;
 const validDraft = $derived(draftCanSave(draft) && !itemsDirty);

 onMount(() => {
  search = data.chat.canSearch;
  try {
   const raw = sessionStorage.getItem('food-chat-seed');
   if (raw) {
    sessionStorage.removeItem('food-chat-seed');
    const seed = JSON.parse(raw) as FoodChatSeed;
 barcode = seed.barcode || '';
    input = `I'd like to discuss this ${seed.kind || 'food'} before saving. Please help me check it and ask any questions you need.\n\n${seed.context}`;
    if (seed.productId) api<Product[]>('/products').then(ps => { product = ps.find(p => p.id === seed.productId) ?? null; barcode = product?.barcode || barcode; }).catch(e => { error = errorText(e); });
   } else {
    const stored = JSON.parse(sessionStorage.getItem(storageKey()) || 'null');
    if (stored && Array.isArray(stored.turns) && stored.turns.every((t: FoodChatTurn) => ['user','assistant'].includes(t.role) && typeof t.content === 'string')) {
     turns = stored.turns; input = stored.input || ''; draft = stored.draft; product = stored.product; saved = stored.saved || ''; barcode = stored.barcode || stored.product?.barcode || ''; itemSnapshot = stored.itemSnapshot || JSON.stringify(stored.draft?.items ?? []);
    }
   }
  } catch { error = 'The previous chat could not be restored. You can start a new one.'; }
  ready = true;
 });
 $effect(() => {
  if (!ready) return;
  try { sessionStorage.setItem(storageKey(), JSON.stringify({turns,input,draft,product,saved,itemSnapshot,barcode})); } catch { /* A full session cache must not block conversation. */ }
 });
 async function send(override?: string) {
  if (!(override || input.trim()) || busy) return false;
  error = ''; busy = true;
  const question = (override || input.trim()) + (draft ? '\n\nCurrent edited draft for discussion: ' + JSON.stringify(draft) : '');
  const next: FoodChatTurn[] = [...turns,{role:'user',content:question}];
  try {
   const reply = await api<{text:string; citations:ChatCitation[]; searched:boolean}>('/food-chat', {body:{messages:conversationPayload(next),search}});
   turns = [...next,{role:'assistant',content:reply.text,citations:reply.citations,searched:reply.searched}];
   if (!override) input = ''; draft = null; saved = ''; saveError = '';
 return true;
  } catch(e) { error = errorText(e); return false; }
  finally { busy = false; }
 }
 async function recalculateMeal() {
 if (await send("Please recalculate nutrition for the changed foods and portions in my current draft, for one serving.")) await prepare('meal');
 }
 async function prepare(kind: 'product' | 'meal') {
  if (busy || !turns.length) return;
  busy = true; error = ''; saved = ''; saveError = '';
  try {
   draft = await api<FoodDraft>('/food-chat/draft',{body:{messages:conversationPayload(turns),kind}});
   draft.macros = {...emptyMacros(),...draft.macros}; draft.items ??= []; itemSnapshot = JSON.stringify(draft.items);
  } catch(e) { error = errorText(e); }
  finally { busy = false; }
 }
 async function saveDraft() {
  if (!draft || !validDraft || busy || saved) return;
  busy = true; saveError = '';
  try {
   const sources = [...new Set(turns.flatMap(t => t.citations?.map(c => c.url).filter(u => safeCitationURL(u)) ?? []))];
   const notes = [draft.notes,...sources].join('\n');
   if (draft.kind === 'product') {
    await api('/products',{body:{...(product ?? {}),name:draft.name,barcode,ingredients:draft.ingredients,basisQuantity:draft.basisQuantity,basisUnit:draft.basisUnit,macros:draft.macros,source:'ai',fromNutritionLabel:false,confidence:'low',notes,rating:undefined}});
    saved = product ? 'Product updated.' : 'Saved to Products.';
   } else {
    await api('/food-chat/meals',{body:{...draft,notes}});
    saved = 'Saved to Meals. Nothing has been added to your daily log.';
   }
   await invalidateAll();
  } catch(e) { saveError = errorText(e); }
  finally { busy = false; }
 }
 function reset() { barcode = ''; turns = []; input = ''; draft = null; product = null; saved = ''; error = ''; saveError = ''; }
</script>

<svelte:head><title>Food chat · Larder</title></svelte:head>
<div class="max-w-3xl">
 <div class="flex items-center justify-between gap-3"><h1 class="text-3xl font-extrabold">Food chat</h1><button class="btn btn-ghost btn-sm" disabled={busy} onclick={reset}>New chat</button></div>
 <p class="text-base-content/70 mt-2 text-sm">Discuss meals and products, check nutrition, and ask follow-up questions. Prepare a draft when you’re ready; nothing goes into your meals, products or daily log until you save.</p>
 {#if !data.chat.configured}<p class="alert alert-warning mt-4">Choose a model in <a class="link" href="/settings">Settings</a> to start chatting.</p>{/if}
 <div class="mt-4 flex flex-wrap items-center gap-3 text-sm">
  <label class="flex items-center gap-2"><input type="checkbox" class="checkbox checkbox-sm" bind:checked={search} disabled={!data.chat.canSearch || busy} />Search the web</label>
  <span class="text-base-content/50">{data.chat.model || 'No model selected'}</span>
 </div>
 <p class="text-base-content/60 mt-1 text-xs">{data.chat.canSearch ? 'Web search uses your configured OpenAI model and API connection. The model must support the Responses API web-search tool; API search charges may apply.' : 'Web search needs an OpenAI Responses-compatible connection. You can still chat without it.'}</p>
 <div class="mt-6 space-y-4" aria-live="polite" aria-label="Conversation">
  {#if !turns.length}<div class="bg-base-200 rounded-box p-4 text-sm"><p class="font-semibold">What would you like to work out?</p><p class="mt-2">Try “Look up the ingredients and nutrition for this vintage cheddar in Ireland” or “Help me build a lunch with chicken and rice—ask me about portions first.”</p></div>{/if}
  {#each turns as turn, i (i)}
   <article class="rounded-box border-base-300 border p-4 {turn.role === 'user' ? 'bg-base-200 ml-6' : 'bg-base-100 mr-2'}">
    <p class="mb-2 text-xs font-bold">{turn.role === 'user' ? 'You' : 'Assistant'}{#if turn.role === 'assistant'}<span class="text-base-content/50 ml-2 font-normal">{turn.searched ? 'Web searched' : 'No web search this turn'}</span>{/if}</p>
    <div class="whitespace-pre-wrap break-words text-sm leading-relaxed">{#each citationParts(turn.content,turn.citations) as part}{#if part.url}<a class="link link-primary" href={part.url} title={part.title} target="_blank" rel="noopener noreferrer">{part.text}</a>{:else}{part.text}{/if}{/each}</div>
    {#if turn.citations?.length}<ul class="mt-3 space-y-1 text-xs">{#each turn.citations as source}{#if safeCitationURL(source.url)}<li><a class="link" href={source.url} target="_blank" rel="noopener noreferrer">{source.title || source.url}</a></li>{/if}{/each}</ul>{/if}
   </article>
  {/each}
  {#if busy}<p class="flex items-center gap-2 text-sm" role="status"><span class="loading loading-spinner loading-sm"></span>Working on it…</p>{/if}
 </div>
 <form class="mt-5" onsubmit={(e) => {e.preventDefault();send();}}>
  <label class="text-sm font-semibold" for="food-chat-question">Your question</label>
  <textarea id="food-chat-question" class="textarea mt-2 w-full" rows="3" maxlength="8000" placeholder="Describe a food, ask for a lookup, or ask a follow-up…" bind:value={input} disabled={busy || !data.chat.configured}></textarea>
  <button class="btn btn-primary mt-2" type="submit" disabled={busy || !input.trim() || !data.chat.configured}>Send</button>
 </form>
 {#if error}<p role="alert" class="alert alert-error mt-3 text-sm">{error}</p>{/if}
 {#if turns.length}
  <div class="mt-5 flex flex-wrap gap-2"><button class="btn btn-sm" disabled={busy} onclick={() => prepare('product')}>Prepare product</button><button class="btn btn-sm" disabled={busy} onclick={() => prepare('meal')}>Prepare meal</button></div>
  <p class="text-base-content/60 mt-2 text-xs">Keep asking questions until the food and portions are clear. Preparing creates an editable draft.</p>
 {/if}
 {#if draft}
  <section class="rounded-box border-primary mt-5 border-2 p-4" aria-label="Review draft">
   <h2 class="text-xl font-bold">Review {draft.kind} draft</h2>
   <p class="text-base-content/70 mt-2 text-sm">AI-assisted details remain provisional. Check sources and portions. {draft.notes}</p>
   <label class="mt-3 block text-sm">Name<input class="input mt-1 w-full" bind:value={draft.name} disabled={!!saved} /></label>
   {#if draft.kind === 'product'}<label class="mt-3 block text-sm">Barcode (optional)<input class="input mt-1 w-full" inputmode="numeric" bind:value={barcode} disabled={!!saved} /></label><label class="mt-3 block text-sm">Ingredients<textarea class="textarea mt-1 w-full" rows="3" bind:value={draft.ingredients} disabled={!!saved}></textarea></label>{/if}
   {#if draft.kind === 'meal'}
    <label class="mt-3 block text-sm">Meal type<select class="select mt-1 w-full" bind:value={draft.mealType} disabled={!!saved}>{#each MEAL_TYPES as type}<option value={type}>{type}</option>{/each}</select></label>
    <p class="mt-3 text-sm font-semibold">Foods and portions for one serving</p>
    {#each draft.items as item, index (index)}<div class="mt-2 grid grid-cols-[1fr_1fr_auto] gap-2"><input class="input input-sm w-full" aria-label="Food {index+1}" bind:value={item.name} disabled={!!saved} /><input class="input input-sm w-full" aria-label="Portion {index+1}" bind:value={item.quantity} disabled={!!saved} /><button class="btn btn-ghost btn-sm" aria-label="Remove food {index+1}" disabled={!!saved} onclick={() => draft!.items.splice(index,1)}>✕</button></div>{/each}
    <button class="btn btn-ghost btn-sm mt-2" disabled={!!saved} onclick={() => draft!.items.push({name:'',quantity:''})}>Add food</button>
    {#if itemsDirty}<p class="text-warning mt-2 text-sm">Foods or portions changed. Recalculate nutrition before saving.</p><button class="btn btn-sm mt-2" disabled={busy} onclick={recalculateMeal}>Recalculate this meal</button>{:else}<p class="text-base-content/60 mt-1 text-xs">If you change foods or portions, recalculate nutrition before saving.</p>{/if}
   {/if}
   <label class="mt-3 flex items-center gap-2 text-sm"><input class="checkbox checkbox-sm" type="checkbox" bind:checked={draft.hasNutrition} disabled={!!saved} />Nutrition available</label>
   {#if draft.hasNutrition}
    <div class="mt-3 flex items-center gap-2 text-sm">Nutrition per <input class="input input-sm w-20" type="number" min="0" step="any" aria-label="Nutrition quantity" bind:value={draft.basisQuantity} disabled={!!saved || draft.kind==='meal'} /><select class="select select-sm w-auto" aria-label="Nutrition units" bind:value={draft.basisUnit} disabled={!!saved || draft.kind==='meal'}><option value="g">g</option><option value="ml">ml</option><option value="serving">serving</option></select></div>
    <div class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">{#each fields as [key,label] (key)}<label class="text-xs">{label}<input class="input input-sm mt-1 w-full" type="number" min="0" step="any" bind:value={draft.macros[key]} disabled={!!saved} /></label>{/each}</div>
   {:else}<p class="text-warning mt-2 text-sm">Nutrition is still missing. Ask a follow-up or enter it before saving.</p>{/if}
   {#if !saved}<button class="btn btn-primary mt-4" disabled={busy || !validDraft} onclick={saveDraft}>{draft.kind==='meal' ? 'Save meal' : product ? 'Update product' : 'Save product'}</button>{/if}
   {#if saveError}<p role="alert" class="text-error mt-2 text-sm">{saveError}</p>{/if}
   {#if saved}<p role="status" class="text-success mt-3 font-semibold">{saved}</p><a class="link mt-2 inline-block" href={draft.kind==='meal' ? '/meals' : '/products'}>View {draft.kind==='meal' ? 'Meals' : 'Products'}</a>{/if}
  </section>
 {/if}
</div>
