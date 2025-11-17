import { writable } from "svelte/store";
import type { User } from "$lib/types";

export const users = writable<User[]>([]);
export const selectedUsers = writable<number[]>([]);
export const loading = writable<boolean>(false);
export const error = writable<string | null>(null);
