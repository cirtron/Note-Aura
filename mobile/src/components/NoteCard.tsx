import React from 'react';
import { View, Text, TouchableOpacity, StyleSheet } from 'react-native';
import type { NoteItem } from '../lib/types';

interface Props {
  note: NoteItem;
  onPress: () => void;
}

const STATUS_COLOR: Record<string, string> = {
  ready: '#10b981',
  processing: '#f59e0b',
  failed: '#ef4444',
};

export function NoteCard({ note, onPress }: Props) {
  const statusColor = STATUS_COLOR[note.status] ?? '#6b7280';
  const date = new Date(note.updated_at).toLocaleDateString();

  return (
    <TouchableOpacity style={styles.card} onPress={onPress} activeOpacity={0.7}>
      <View style={styles.header}>
        <Text style={styles.title} numberOfLines={2}>
          {note.title || '(Untitled)'}
        </Text>
        <View style={[styles.badge, { backgroundColor: statusColor + '22' }]}>
          <Text style={[styles.badgeText, { color: statusColor }]}>{note.status}</Text>
        </View>
      </View>
      {note.summary ? (
        <Text style={styles.summary} numberOfLines={2}>{note.summary}</Text>
      ) : null}
      <View style={styles.footer}>
        {note.category ? (
          <Text style={styles.category}>📁 {note.category}</Text>
        ) : null}
        <Text style={styles.date}>{date}</Text>
      </View>
      {note.tags.length > 0 && (
        <Text style={styles.tags} numberOfLines={1}>
          {note.tags.map((t) => `#${t}`).join('  ')}
        </Text>
      )}
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: '#fff',
    borderRadius: 10,
    padding: 14,
    marginHorizontal: 12,
    marginVertical: 5,
    shadowColor: '#000',
    shadowOpacity: 0.06,
    shadowRadius: 6,
    shadowOffset: { width: 0, height: 2 },
    elevation: 2,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: 8,
  },
  title: {
    flex: 1,
    fontSize: 15,
    fontWeight: '600',
    color: '#111827',
  },
  badge: {
    borderRadius: 10,
    paddingHorizontal: 7,
    paddingVertical: 2,
  },
  badgeText: {
    fontSize: 11,
    fontWeight: '600',
    textTransform: 'capitalize',
  },
  summary: {
    marginTop: 5,
    fontSize: 13,
    color: '#6b7280',
    lineHeight: 18,
  },
  footer: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: 8,
  },
  category: {
    fontSize: 12,
    color: '#d97706',
    fontWeight: '500',
  },
  date: {
    fontSize: 12,
    color: '#9ca3af',
  },
  tags: {
    marginTop: 4,
    fontSize: 12,
    color: '#6366f1',
  },
});
