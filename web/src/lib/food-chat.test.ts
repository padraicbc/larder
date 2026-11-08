import { describe, expect, it } from 'vitest';
import { citationParts, conversationPayload, draftCanSave, safeCitationURL, type FoodDraft } from './food-chat.ts';
const meal: FoodDraft = {kind:'meal',name:'Rice lunch',ingredients:'Rice',hasNutrition:true,basisQuantity:1,basisUnit:'serving',macros:{calories:500,protein:35,carbs:60,fat:12,saturatedFat:3,fiber:4},items:[{name:'Cooked rice',quantity:'150 g'}],mealType:'lunch',notes:'AI-assisted'};
describe('food chat review and citations', () => {
 it('renders valid native citation offsets, including Unicode, without raw HTML', () => {
  expect(citationParts('🧀 cheese source', [{url:'https://maker.example/food',title:'Maker',start:9,end:15}])).toEqual([{text:'🧀 cheese '},{text:'source',url:'https://maker.example/food',title:'Maker'}]);
  expect(citationParts('<script>bad</script>', [{url:'javascript:alert(1)',title:'bad',start:0,end:8}])).toEqual([{text:'<script>bad</script>'}]);
  expect(safeCitationURL('data:text/html,bad')).toBeNull();
  expect(safeCitationURL('https://user:secret@example.com')).toBeNull();
 });
 it('keeps sources with earlier replies when sending follow-up history', () => {
  const history=conversationPayload([{role:'user',content:'Lookup cheese'},{role:'assistant',content:'27 g saturated fat',citations:[{url:'https://maker.example/food',title:'Maker',start:0,end:2}]},{role:'user',content:'For 30 g?'}]);
  expect(history[1].content).toContain('https://maker.example/food');expect(history[2].content).toBe('For 30 g?');
 });
 it('blocks saving missing portions, missing nutrition and invalid saturated fat', () => {
  expect(draftCanSave(meal)).toBe(true);
  expect(draftCanSave({...meal,hasNutrition:false})).toBe(false);
  expect(draftCanSave({...meal,items:[{name:'Rice',quantity:''}]})).toBe(false);
  expect(draftCanSave({...meal,macros:{...meal.macros,saturatedFat:13}})).toBe(false);
  expect(draftCanSave({...meal,basisQuantity:100,basisUnit:'g'})).toBe(false);
  expect(draftCanSave({...meal,kind:'product',basisQuantity:100,basisUnit:'g',ingredients:'',items:[]})).toBe(true);
 });
});
