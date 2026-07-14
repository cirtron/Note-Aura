import React from 'react';
import { ScrollView, StyleSheet } from 'react-native';
import Markdown from 'react-native-markdown-display';

interface Props {
  content: string;
}

export function MarkdownView({ content }: Props) {
  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Markdown style={markdownStyles}>{content}</Markdown>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#fff' },
  content: { padding: 16 },
});

const markdownStyles = {
  body: { fontSize: 15, lineHeight: 24, color: '#1f2937' },
  heading1: { fontSize: 22, fontWeight: '700' as const, marginBottom: 8, color: '#111827' },
  heading2: { fontSize: 19, fontWeight: '600' as const, marginBottom: 6, color: '#111827' },
  heading3: { fontSize: 17, fontWeight: '600' as const, marginBottom: 4, color: '#111827' },
  code_block: { backgroundColor: '#f3f4f6', borderRadius: 6, padding: 10, fontFamily: 'monospace' },
  code_inline: { backgroundColor: '#f3f4f6', borderRadius: 4, paddingHorizontal: 4, fontFamily: 'monospace' },
  blockquote: { borderLeftWidth: 3, borderLeftColor: '#d1d5db', paddingLeft: 12, color: '#6b7280' },
  link: { color: '#4f46e5' },
  bullet_list_icon: { color: '#4f46e5' },
};
