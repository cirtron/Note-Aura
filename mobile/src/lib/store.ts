import { create } from 'zustand';
import * as SecureStore from 'expo-secure-store';

const TOKEN_KEY = 'note_aura_token';
const BASE_URL_KEY = 'note_aura_base_url';

interface AuthState {
  token: string | null;
  baseURL: string;
  userEmail: string;
  isAdmin: boolean;

  setAuth: (token: string, email: string, isAdmin: boolean) => Promise<void>;
  setBaseURL: (url: string) => Promise<void>;
  logout: () => Promise<void>;
  loadFromStorage: () => Promise<void>;
}

export const useStore = create<AuthState>((set) => ({
  token: null,
  baseURL: '',
  userEmail: '',
  isAdmin: false,

  setAuth: async (token, email, isAdmin) => {
    await SecureStore.setItemAsync(TOKEN_KEY, token);
    set({ token, userEmail: email, isAdmin });
  },

  setBaseURL: async (url) => {
    await SecureStore.setItemAsync(BASE_URL_KEY, url);
    set({ baseURL: url });
  },

  logout: async () => {
    await SecureStore.deleteItemAsync(TOKEN_KEY);
    set({ token: null, userEmail: '', isAdmin: false });
  },

  loadFromStorage: async () => {
    const token = await SecureStore.getItemAsync(TOKEN_KEY);
    const baseURL = await SecureStore.getItemAsync(BASE_URL_KEY);
    set({ token: token ?? null, baseURL: baseURL ?? '' });
  },
}));
