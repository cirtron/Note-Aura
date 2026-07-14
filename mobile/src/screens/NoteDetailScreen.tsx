import React, { useCallback, useEffect, useState } from 'react';
import {
  View, Text, TouchableOpacity, StyleSheet, Alert, ActivityIndicator, ScrollView,
} from 'react-native';
import { api } from '../lib/api';
import { MarkdownView } from '../components/MarkdownView';
import { TagChip } from '../components/TagChip';
import type { NoteDetail } from '../lib/types';

const STATUS_COLOR: Record<string, string> = {
  ready: '#10b981',
  processing: '#f59e0b',
  failed: '#ef4444',
};

export function NoteDetailScreen({ route, navigation }: { route: any; navigation: any }) {
  const { id } = route.params;
  const [note, setNote] = useState<NoteDetail | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await api.getNote(id);
      setNote(data);
      navigation.setOptions({ title: data.title || '(Untitled)' });
    } catch {
      Alert.alert('Error', 'Could not load note.');
      navigation.goBack();
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const handleDelete = () => {
    Alert.alert('Delete note', 'This cannot be undone.', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete', style: 'destructive', onPress: async () => {
          await api.deleteNote(id);
          navigation.goBack();
        },
      },
    ]);
  };

  useEffect(() => {
    if (!note) return;
    navigation.setOptions({
      headerRight: () => (
        <View style={styles.headerBtns}>
          <TouchableOpacity onPress={() => navigation.push('NoteEdit', { id })} style={styles.headerBtn}>
            <Text style={styles.headerBtnTxt}>Edit</Text>
          </TouchableOpacity>
          <TouchableOpacity onPress={handleDelete} style={styles.headerBtn}>
            <Text style={[styles.headerBtnTxt, { color: '#ef4444' }]}>Delete</Text>
          </TouchableOpacity>
        </View>
      ),
    });
  }, [note]);

  if (loading) {
    return <ActivityIndicator style={styles.loader} size="large" color="#4f46e5" />;
  }
  if (!note) return null;

  const statusColor = STATUS_COLOR[note.status] ?? '#6b7280';

  return (
    <View style={styles.container}>
      {/* Meta header */}
      <ScrollView style={styles.metaScroll}>
        <View style={styles.meta}>
          <View style={[styles.badge, { backgroundColor: statusColor + '22' }]}>
            <Text style={[styles.badgeText, { color: statusColor }]}>{note.status}</Text>
          </View>
          {note.category ? <TagChip label={note.category} variant="category" /> : null}
          {note.tags.map((t) => <TagChip key={t} label={t} />)}
        </View>
      </ScrollView>

      {note.summary ? (
        <View style={styles.summaryBox}>
          <Text style={styles.summaryLabel}>AI Summary</Text>
          <Text style={styles.summary}>{note.summary}</Text>
        </View>
      ) : null}

      {note.status === 'processing' ? (
        <View style={styles.processingBox}>
          <ActivityIndicator color="#f59e0b" />
          <Text style={styles.processingTxt}>AI is processing…</Text>
        </View>
      ) : null}

      <MarkdownView content={note.body_md} />
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#fff' },
  loader: { flex: 1 },
  metaScroll: { maxHeight: 52, borderBottomWidth: 1, borderBottomColor: '#e5e7eb' },
  meta: { flexDirection: 'row', flexWrap: 'wrap', paddingHorizontal: 12, paddingVertical: 8, gap: 4 },
  badge: { borderRadius: 10, paddingHorizontal: 8, paddingVertical: 3, alignSelf: 'flex-start' },
  badgeText: { fontSize: 12, fontWeight: '600', textTransform: 'capitalize' },
  summaryBox: { backgroundColor: '#f0fdf4', padding: 12, borderBottomWidth: 1, borderBottomColor: '#d1fae5' },
  summaryLabel: { fontSize: 11, fontWeight: '700', color: '#059669', marginBottom: 3, textTransform: 'uppercase', letterSpacing: 0.5 },
  summary: { fontSize: 14, color: '#065f46', lineHeight: 20 },
  processingBox: { flexDirection: 'row', alignItems: 'center', gap: 8, padding: 12, backgroundColor: '#fffbeb', borderBottomWidth: 1, borderBottomColor: '#fef3c7' },
  processingTxt: { fontSize: 13, color: '#92400e' },
  headerBtns: { flexDirection: 'row', gap: 12 },
  headerBtn: { paddingHorizontal: 4 },
  headerBtnTxt: { color: '#4f46e5', fontWeight: '600', fontSize: 15 },
});
