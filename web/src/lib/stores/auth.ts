import { writable, derived } from "svelte/store";
import { browser } from "$app/environment";
import { api } from "$lib/api/client";

export interface User {
  name: string;
  email: string;
}

export interface AuthState {
  token: string | null;
  expiresAt: string | null;
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

const STORAGE_KEY = "auth_state";

function getInitialState(): AuthState {
  if (browser) {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      try {
        const parsed = JSON.parse(stored);
        // Check if token is expired
        if (parsed.expiresAt && new Date(parsed.expiresAt) > new Date()) {
          return {
            ...parsed,
            isAuthenticated: true,
            isLoading: false,
          };
        }
      } catch (e) {
        console.error("Failed to parse stored auth state:", e);
      }
    }
  }
  return {
    token: null,
    expiresAt: null,
    user: null,
    isAuthenticated: false,
    isLoading: false,
  };
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>(getInitialState());

  return {
    subscribe,
    login: (token: string, expiresAt: string, user: User) => {
      const newState: AuthState = {
        token,
        expiresAt,
        user,
        isAuthenticated: true,
        isLoading: false,
      };
      if (browser) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(newState));
      }
      set(newState);
    },
    logout: () => {
      if (browser) {
        localStorage.removeItem(STORAGE_KEY);
      }
      set({
        token: null,
        expiresAt: null,
        user: null,
        isAuthenticated: false,
        isLoading: false,
      });
    },
    setLoading: (isLoading: boolean) => {
      update((state) => ({ ...state, isLoading }));
    },
    refreshToken: async () => {
      const state = getInitialState();
      if (!state.token) {
        throw new Error("No token available");
      }

      const response = await api.refreshToken();

      if (response.error) {
        throw new Error(response.error);
      }

      if (!response.data) {
        throw new Error("No data received from refresh");
      }

      const data = response.data;
      const newState: AuthState = {
        token: data.token,
        expiresAt: data.expires_at,
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
      };

      if (browser) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(newState));
      }
      set(newState);
      return data.token;
    },
    getCurrentUser: async () => {
      const response = await api.getCurrentUser();

      if (response.error) {
        throw new Error(response.error);
      }

      if (!response.data) {
        throw new Error("No user data received");
      }

      update((state) => ({
        ...state,
        user: response.data!,
      }));

      return response.data;
    },
  };
}

export const auth = createAuthStore();

// Derived store for just the token
export const authToken = derived(auth, ($auth) => $auth.token);

// Derived store for user
export const currentUser = derived(auth, ($auth) => $auth.user);
