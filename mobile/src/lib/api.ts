import axios from 'axios';
import { useStore } from './store';
import type { LoginResponse, NotesPage, NoteDetail, TagItem, CategoryItem } from './types';

export const client = axios.create({ timeout: 15000 });

// Inject baseURL + Bearer token before every request.
client.interceptors.request.use((config) => {
  const { baseURL, token } = useStore.getState();
  if (baseURL) config.baseURL = baseURL.replace(/\/$/, '');
  if (token) config.headers['Authorization'] = `Bearer ${token}`;
  return config;
});

// On 401, clear auth and trigger navigation to Login via store.
client.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      useStore.getState().logout();
    }
    return Promise.reject(err);
  },
);

export const api = {
  login: (email: string, password: string, deviceName = 'mobile') =>
    client.post<LoginResponse>('/api/auth/login', { email, password, device_name: deviceName }).then((r) => r.data),

  logout: () => client.post('/api/auth/logout'),

  getNotes: (params?: {
    page?: number;
    per_page?: number;
    q?: string;
    tag?: string;
    category?: string;
    sort?: string;
  }) => client.get<NotesPage>('/api/notes', { params }).then((r) => r.data),

  getNote: (id: number) => client.get<NoteDetail>(`/api/notes/${id}`).then((r) => r.data),

  createNote: (body: {
    title?: string;
    body_md?: string;
    tags?: string[];
    category?: string;
    source_type?: string;
  }) => client.post<NoteDetail>('/api/notes', body).then((r) => r.data),

  updateNote: (id: number, body: {
    title?: string;
    body_md?: string;
    tags?: string[];
    category?: string;
  }) => client.put<NoteDetail>(`/api/notes/${id}`, body).then((r) => r.data),

  deleteNote: (id: number) => client.delete(`/api/notes/${id}`).then((r) => r.data),

  getTags: () => client.get<TagItem[]>('/api/tags').then((r) => r.data),

  getCategories: () => client.get<CategoryItem[]>('/api/categories').then((r) => r.data),
};
