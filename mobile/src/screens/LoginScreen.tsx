import React, { useState } from 'react';
import {
  View, Text, TextInput, TouchableOpacity, StyleSheet,
  KeyboardAvoidingView, Platform, ScrollView, Alert, ActivityIndicator,
} from 'react-native';
import { api } from '../lib/api';
import { useStore } from '../lib/store';

export function LoginScreen({ navigation }: { navigation: any }) {
  const { setAuth, setBaseURL, baseURL } = useStore();
  const [serverURL, setServerURL] = useState(baseURL || 'http://');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = async () => {
    const url = serverURL.trim().replace(/\/$/, '');
    if (!url || !email.trim() || !password) {
      Alert.alert('Error', 'Please fill in all fields.');
      return;
    }
    setLoading(true);
    try {
      await setBaseURL(url);
      const data = await api.login(email.trim().toLowerCase(), password);
      await setAuth(data.token, data.user.email, data.user.is_admin);
      navigation.replace('Main');
    } catch (err: any) {
      const msg = err.response?.data?.error ?? err.message ?? 'Login failed';
      Alert.alert('Login failed', msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <KeyboardAvoidingView style={styles.flex} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerStyle={styles.container} keyboardShouldPersistTaps="handled">
        <Text style={styles.logo}>📝</Text>
        <Text style={styles.title}>Note-Aura</Text>
        <Text style={styles.subtitle}>Sign in to your server</Text>

        <View style={styles.form}>
          <Text style={styles.label}>Server URL</Text>
          <TextInput
            style={styles.input}
            value={serverURL}
            onChangeText={setServerURL}
            placeholder="https://your-server.com"
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="url"
          />

          <Text style={styles.label}>Email</Text>
          <TextInput
            style={styles.input}
            value={email}
            onChangeText={setEmail}
            placeholder="you@example.com"
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="email-address"
          />

          <Text style={styles.label}>Password</Text>
          <TextInput
            style={styles.input}
            value={password}
            onChangeText={setPassword}
            placeholder="••••••••"
            secureTextEntry
          />

          <TouchableOpacity style={styles.button} onPress={handleLogin} disabled={loading}>
            {loading ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text style={styles.buttonText}>Sign in</Text>
            )}
          </TouchableOpacity>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1, backgroundColor: '#f9fafb' },
  container: { flexGrow: 1, justifyContent: 'center', padding: 24 },
  logo: { fontSize: 48, textAlign: 'center', marginBottom: 8 },
  title: { fontSize: 28, fontWeight: '700', textAlign: 'center', color: '#111827' },
  subtitle: { fontSize: 15, textAlign: 'center', color: '#6b7280', marginBottom: 32 },
  form: { backgroundColor: '#fff', borderRadius: 12, padding: 20, shadowColor: '#000', shadowOpacity: 0.05, shadowRadius: 8, elevation: 2 },
  label: { fontSize: 13, fontWeight: '600', color: '#374151', marginBottom: 6, marginTop: 12 },
  input: { borderWidth: 1, borderColor: '#d1d5db', borderRadius: 8, paddingHorizontal: 12, paddingVertical: 10, fontSize: 15, color: '#111827', backgroundColor: '#f9fafb' },
  button: { backgroundColor: '#4f46e5', borderRadius: 8, paddingVertical: 13, alignItems: 'center', marginTop: 20 },
  buttonText: { color: '#fff', fontSize: 16, fontWeight: '600' },
});
