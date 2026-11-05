type Toast = { id: number; text: string; kind: 'success' | 'error' | 'info' };

export const toasts = $state<Toast[]>([]);
let next = 0;

export function toast(text: string, kind: Toast['kind'] = 'success', ms = 3500) {
	const id = next++;
	toasts.push({ id, text, kind });
	setTimeout(() => {
		const i = toasts.findIndex((t) => t.id === id);
		if (i >= 0) toasts.splice(i, 1);
	}, ms);
}
