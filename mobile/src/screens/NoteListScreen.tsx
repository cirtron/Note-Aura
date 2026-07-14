import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  View, Text, TextInput, FlatList, RefreshControl,
  ActivityIndicator, StyleSheet, ScrollView, TouchableOpacity,
} from 'react-native';
import { api } from '../lib/api';
import { NoteCard } from '../components/NoteCard';
import { TagChip } from '../components/TagChip';
import type { NoteItem, TagItem } from '../lib/types';

const PER_PAGE = 20;

export function NoteListScreen({ navigation }: { navigation: any }) {
  const [notes, setNotes] = useState<NoteItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [query, setQuery] = useState('');
  const [activeTag, setActiveTag] = useState('');
  const [tags, setTags] = useState<TagItem[]>([]);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchPage = useCallback(async (q: string, tag: string, pg: number, append = false) => {
    try {
      const data = await api.getNotes({ q, tag, page: pg, per_page: PER_PAGE });
      setNotes((prev) => append ? [...prev, ...data.notes] : data.notes);
      setTotal(data.total);
      setPage(pg);
    } catch {}
  }, []);

  const load = useCallback(async (q: string, tag: string) => {
    setLoading(true);
    await fetchPage(q, tag, 1, false);
    setLoading(false);
  }, [fetchPage]);

  useEffect(() => {
    load(query, activeTag);
    api.getTags().then(setTags).catch(() => {});
  }, []);

  const onRefresh = async () => {
    setRefreshing(true);
    await fetchPage(query, activeTag, 1, false);
    setRefreshing(false);
  };

  const onQueryChange = (text: string) => {
    setQuery(text);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => load(text, activeTag), 350);
  };

  const onTagPress = (tag: string) => {
    const next = activeTag === tag ? '' : tag;
    setActiveTag(next);
    load(query, next);
  };

  const loadMore = async () => {
    if (notes.length >= total || loadingMore) return;
    setLoadingMore(true);
    await fetchPage(query, activeTag, page + 1, true);
    setLoadingMore(false);
  };

  const hasMore = notes.length < total;

  return (
    <View style={styles.container}>
      {/* Search bar */}
      <View style={styles.searchRow}>
        <TextInput
          style={styles.search}
          placeholder="Search notes…"
          value={query}
          onChangeText={onQueryChange}
          clearButtonMode="while-editing"
        />
      </View>

      {/* Tag filter chips */}
      {tags.length > 0 && (
        <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.tagRow} contentContainerStyle={styles.tagContent}>
          {tags.slice(0, 20).map((t) => (
            <TagChip key={t.name} label={t.name} active={activeTag === t.name} onPress={() => onTagPress(t.name)} />
          ))}
        </ScrollView>
      )}

      {loading ? (
        <ActivityIndicator style={styles.loader} size="large" color="#4f46e5" />
      ) : (
        <FlatList
          data={notes}
          keyExtractor={(n) => String(n.id)}
          renderItem={({ item }) => (
            <NoteCard note={item} onPress={() => navigation.push('NoteDetail', { id: item.id })} />
          )}
          refreshControl={<RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor="#4f46e5" />}
          ListEmptyComponent={<Text style={styles.empty}>No notes found.</Text>}
          ListFooterComponent={
            hasMore ? (
              <TouchableOpacity style={styles.moreBtn} onPress={loadMore} disabled={loadingMore}>
                {loadingMore ? <ActivityIndicator color="#4f46e5" /> : <Text style={styles.moreTxt}>Load more</Text>}
              </TouchableOpacity>
            ) : null
          }
          contentContainerStyle={{ paddingBottom: 20 }}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#f9fafb' },
  searchRow: { paddingHorizontal: 12, paddingVertical: 8, backgroundColor: '#fff', borderBottomWidth: 1, borderBottomColor: '#e5e7eb' },
  search: { backgroundColor: '#f3f4f6', borderRadius: 8, paddingHorizontal: 12, paddingVertical: 8, fontSize: 15 },
  tagRow: { maxHeight: 44, backgroundColor: '#fff', borderBottomWidth: 1, borderBottomColor: '#e5e7eb' },
  tagContent: { paddingHorizontal: 10, paddingVertical: 8, flexDirection: 'row' },
  loader: { marginTop: 60 },
  empty: { textAlign: 'center', color: '#9ca3af', marginTop: 60, fontSize: 15 },
  moreBtn: { alignItems: 'center', paddingVertical: 14 },
  moreTxt: { color: '#4f46e5', fontWeight: '600' },
});
