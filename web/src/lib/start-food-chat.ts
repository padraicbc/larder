import { goto } from '$app/navigation';
import type { FoodChatSeed } from './food-chat.ts';
export async function startFoodChat(seed: FoodChatSeed) {
 sessionStorage.setItem('food-chat-seed', JSON.stringify(seed));
 await goto('/chat');
}
