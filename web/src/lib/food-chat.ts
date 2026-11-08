import type { Macros, MealItem, MealType } from './types.ts';
export interface ChatCitation { url: string; title: string; start: number; end: number }
export interface FoodChatTurn { role: 'user' | 'assistant'; content: string; citations?: ChatCitation[]; searched?: boolean }
export interface FoodDraft { kind: 'product' | 'meal'; name: string; ingredients: string; hasNutrition: boolean; basisQuantity: number; basisUnit: 'g' | 'ml' | 'serving'; macros: Macros; items: MealItem[]; mealType: MealType; notes: string }
export interface FoodChatSeed { context: string; kind?: 'product' | 'meal'; productId?: string; barcode?: string }

export function conversationPayload(turns: FoodChatTurn[]) {
 return turns.map(t => ({ role: t.role, content: t.content + (t.citations?.length ? '\nSources attached to this reply:\n' + [...new Set(t.citations.map(c => c.url))].join('\n') : '') }));
}
export function safeCitationURL(raw: string): string | null {
 try { const u = new URL(raw); return ['https:', 'http:'].includes(u.protocol) && !u.username && !u.password ? u.href : null; } catch { return null; }
}
export function citationParts(text: string, citations: ChatCitation[] = []) {
 const chars = Array.from(text);
 const parts: { text: string; url?: string; title?: string }[] = [];
 let cursor = 0;
 for (const c of [...citations].sort((a,b) => a.start - b.start)) {
  const url = safeCitationURL(c.url);
  if (!url || !Number.isInteger(c.start) || !Number.isInteger(c.end) || c.start < cursor || c.end <= c.start || c.end > chars.length) continue;
  if (cursor < c.start) parts.push({ text: chars.slice(cursor,c.start).join('') });
  parts.push({ text: chars.slice(c.start,c.end).join(''), url, title: c.title }); cursor = c.end;
 }
 if (cursor < chars.length) parts.push({ text: chars.slice(cursor).join('') });
 return parts;
}
export function draftCanSave(d: FoodDraft | null): boolean {
 if (!d || !d.name.trim() || !d.hasNutrition || !Number.isFinite(d.basisQuantity) || d.basisQuantity <= 0) return false;
 if (!['g','ml','serving'].includes(d.basisUnit)) return false;
 const values = [d.macros.calories,d.macros.protein,d.macros.carbs,d.macros.fat,d.macros.fiber];
 if (!values.every(v => Number.isFinite(v) && v >= 0) || !values.some(v => v > 0)) return false;
 if (d.macros.saturatedFat != null && (!Number.isFinite(d.macros.saturatedFat) || d.macros.saturatedFat < 0 || d.macros.saturatedFat > d.macros.fat)) return false;
 return d.kind === 'product' || (d.basisUnit === 'serving' && d.basisQuantity === 1 && ['breakfast','lunch','dinner','snack'].includes(d.mealType) && d.items.length > 0 && d.items.every(i => i.name.trim() && i.quantity.trim()));
}
