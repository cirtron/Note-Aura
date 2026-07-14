export interface NoteItem {
  id: number;
  title: string;
  summary: string;
  status: 'processing' | 'ready' | 'failed';
  source_type: string;
  tags: string[];
  category: string;
  created_at: string;
  updated_at: string;
}

export interface NoteDetail extends NoteItem {
  body_md: string;
  source_ref?: string;
  error?: string;
  ai_ms?: number;
}

export interface TagItem {
  name: string;
  count: number;
}

export interface CategoryItem {
  name: string;
  count: number;
}

export interface NotesPage {
  notes: NoteItem[];
  total: number;
  page: number;
  per_page: number;
}

export interface LoginResponse {
  token: string;
  user: {
    id: number;
    email: string;
    is_admin: boolean;
  };
}

export type RootStackParamList = {
  Login: undefined;
  Main: undefined;
  NoteDetail: { id: number };
  NoteEdit: { id?: number };
};

export type TabParamList = {
  Notes: undefined;
  New: undefined;
  Settings: undefined;
};
