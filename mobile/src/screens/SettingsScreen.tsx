import React from 'react';
import { View, Text, TouchableOpacity, StyleSheet, Alert, ScrollView } from 'react-native';
import { api } from '../lib/api';
import { useStore } from '../lib/store';

export function SettingsScreen() {
  const { userEmail, baseURL, isAdmin, logout } = useStore();

  const handleLogout = () => {
    Alert.alert('Sign out', 'Are you sure you want to sign out?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Sign out', style: 'destructive', onPress: async () => {
          try { await api.logout(); } catch {}
          await logout();
        },
      },
    ]);
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.heading}>Account</Text>

      <View style={styles.card}>
        <Row label="Email" value={userEmail} />
        <Row label="Server" value={baseURL} />
        {isAdmin && <Row label="Role" value="Admin" />}
      </View>

      <TouchableOpacity style={styles.logoutBtn} onPress={handleLogout}>
        <Text style={styles.logoutTxt}>Sign out</Text>
      </TouchableOpacity>

      <Text style={styles.version}>Note-Aura Mobile</Text>
    </ScrollView>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <View style={rowStyles.row}>
      <Text style={rowStyles.label}>{label}</Text>
      <Text style={rowStyles.value} numberOfLines={1}>{value}</Text>
    </View>
  );
}

const rowStyles = StyleSheet.create({
  row: { flexDirection: 'row', justifyContent: 'space-between', paddingVertical: 10, borderBottomWidth: 1, borderBottomColor: '#f3f4f6' },
  label: { fontSize: 14, color: '#6b7280', fontWeight: '500' },
  value: { fontSize: 14, color: '#111827', flex: 1, textAlign: 'right', marginLeft: 12 },
});

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#f9fafb' },
  content: { padding: 16 },
  heading: { fontSize: 13, fontWeight: '700', color: '#6b7280', textTransform: 'uppercase', letterSpacing: 0.8, marginBottom: 8, marginTop: 8 },
  card: { backgroundColor: '#fff', borderRadius: 10, paddingHorizontal: 16, marginBottom: 24, shadowColor: '#000', shadowOpacity: 0.04, shadowRadius: 4, elevation: 1 },
  logoutBtn: { backgroundColor: '#fef2f2', borderWidth: 1, borderColor: '#fecaca', borderRadius: 8, paddingVertical: 13, alignItems: 'center', marginBottom: 24 },
  logoutTxt: { color: '#ef4444', fontWeight: '600', fontSize: 15 },
  version: { textAlign: 'center', fontSize: 12, color: '#d1d5db' },
});
