import React, { useEffect, useState } from 'react';
import {
  View, Text, TextInput, TouchableOpacity, StyleSheet,
  KeyboardAvoidingView, Platform, ScrollView, Alert, ActivityIndicator,
} from 'react-native';
import { api } from '../lib/api';

export function NoteEditScreen({ route, navigation }: { route: any; navigation: any }) {
  const noteId = route.params?.id;
  const isEdit = !!noteId;

  const [title, setTitle] = useState('');
  const [bodyMd, setBodyMd] = useState('');
  const [tags, setTags] = useState('');
  const [category, setCategory] = useState('');
  const [loading, setLoading] = useState(isEdit);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    navigation.setOptions({ title: isEdit ? 'Edit Note' : 'New Note' });
    if (!isEdit) return;
    api.getNote(noteId).then((n) => {
      setTitle(n.title);
      setBodyMd(n.body_md);
      setTags(n.tags.join(', '));
      setCategory(n.category);
      setLoading(false);
    }).catch(() => {
      Alert.alert('Error', 'Could not load note.');
      navigation.goBack();
    });
  }, [noteId]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const tagList = tags.split(',').map((t) => t.trim()).filter(Boolean);
      if (isEdit) {
        await api.updateNote(noteId, { title, body_md: bodyMd, tags: tagList, category });
        navigation.goBack();
      } else {
        const n = await api.createNote({ title, body_md: bodyMd, tags: tagList, category });
        navigation.replace('NoteDetail', { id: n.id });
      }
    } catch (err: any) {
      Alert.alert('Error', err.response?.data?.error ?? 'Could not save note.');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <ActivityIndicator style={styles.loader} size="large" color="#4f46e5" />;
  }

  return (
    <KeyboardAvoidingView style={styles.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerStyle={styles.container} keyboardShouldPersistTaps="handled">
        <Text style={styles.label}>Title</Text>
        <TextInput
          style={styles.input}
          value={title}
          onChangeText={setTitle}
          placeholder="Title (leave blank for AI to generate)"
        />

        <Text style={styles.label}>Category</Text>
        <TextInput
          style={styles.input}
          value={category}
          onChangeText={setCategory}
          placeholder="Category (e.g. Work/Projects)"
        />

        <Text style={styles.label}>Tags (comma-separated)</Text>
        <TextInput
          style={styles.input}
          value={tags}
          onChangeText={setTags}
          placeholder="tag1, tag2, tag3"
          autoCapitalize="none"
        />

        <Text style={styles.label}>Content (Markdown)</Text>
        <TextInput
          style={[styles.input, styles.bodyInput]}
          value={bodyMd}
          onChangeText={setBodyMd}
          placeholder="Write your note in Markdown…"
          multiline
          textAlignVertical="top"
        />

        <TouchableOpacity style={styles.saveBtn} onPress={handleSave} disabled={saving}>
          {saving ? (
            <ActivityIndicator color="#fff" />
          ) : (
            <Text style={styles.saveBtnTxt}>{isEdit ? 'Save changes' : 'Create note'}</Text>
          )}
        </TouchableOpacity>

        {!isEdit && (
          <Text style={styles.hint}>
            💡 Leave title blank and AI will generate one for you.
          </Text>
        )}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1, backgroundColor: '#f9fafb' },
  loader: { flex: 1 },
  container: { padding: 16, paddingBottom: 40 },
  label: { fontSize: 13, fontWeight: '600', color: '#374151', marginBottom: 6, marginTop: 14 },
  input: {
    borderWidth: 1, borderColor: '#d1d5db', borderRadius: 8,
    paddingHorizontal: 12, paddingVertical: 10, fontSize: 15,
    color: '#111827', backgroundColor: '#fff',
  },
  bodyInput: { minHeight: 200, lineHeight: 22 },
  saveBtn: { backgroundColor: '#4f46e5', borderRadius: 8, paddingVertical: 13, alignItems: 'center', marginTop: 20 },
  saveBtnTxt: { color: '#fff', fontSize: 16, fontWeight: '600' },
  hint: { marginTop: 12, textAlign: 'center', fontSize: 13, color: '#9ca3af' },
});
