import { writable } from "svelte/store";
import type { Tag } from "$lib/types";

export const tags = writable<Tag[]>([]);
export const loading = writable<boolean>(false);
export const error = writable<string | null>(null);
