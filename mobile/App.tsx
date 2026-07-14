import React, { useEffect, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';

import { useStore } from './src/lib/store';
import type { RootStackParamList, TabParamList } from './src/lib/types';

import { LoginScreen } from './src/screens/LoginScreen';
import { NoteListScreen } from './src/screens/NoteListScreen';
import { NoteDetailScreen } from './src/screens/NoteDetailScreen';
import { NoteEditScreen } from './src/screens/NoteEditScreen';
import { SettingsScreen } from './src/screens/SettingsScreen';

const Stack = createNativeStackNavigator<RootStackParamList>();
const Tab = createBottomTabNavigator<TabParamList>();

function MainTabs() {
  return (
    <Tab.Navigator
      screenOptions={{
        tabBarActiveTintColor: '#4f46e5',
        tabBarInactiveTintColor: '#9ca3af',
        headerStyle: { backgroundColor: '#fff' },
        headerTintColor: '#111827',
        headerTitleStyle: { fontWeight: '700' },
      }}
    >
      <Tab.Screen
        name="Notes"
        component={NoteListScreen}
        options={{ title: 'Notes', tabBarLabel: 'Notes', tabBarIcon: ({ color }) => <TabIcon icon="📝" color={color} /> }}
      />
      <Tab.Screen
        name="New"
        component={NewNoteWrapper}
        options={{ title: 'New Note', tabBarLabel: 'New', tabBarIcon: ({ color }) => <TabIcon icon="✏️" color={color} /> }}
      />
      <Tab.Screen
        name="Settings"
        component={SettingsScreen}
        options={{ title: 'Settings', tabBarLabel: 'Settings', tabBarIcon: ({ color }) => <TabIcon icon="⚙️" color={color} /> }}
      />
    </Tab.Navigator>
  );
}

// Wrap NoteEditScreen (create mode) so it can be a Tab screen.
function NewNoteWrapper({ navigation }: any) {
  return <NoteEditScreen route={{ params: {} } as any} navigation={navigation} />;
}

function TabIcon({ icon }: { icon: string; color: string }) {
  const { Text } = require('react-native');
  return <Text style={{ fontSize: 20 }}>{icon}</Text>;
}

export default function App() {
  const { token, loadFromStorage } = useStore();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    loadFromStorage().finally(() => setReady(true));
  }, []);

  if (!ready) {
    return (
      <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}>
        <ActivityIndicator size="large" color="#4f46e5" />
      </View>
    );
  }

  return (
    <SafeAreaProvider>
      <StatusBar style="auto" />
      <NavigationContainer>
        <Stack.Navigator
          initialRouteName={token ? 'Main' : 'Login'}
          screenOptions={{
            headerStyle: { backgroundColor: '#fff' },
            headerTintColor: '#111827',
            headerTitleStyle: { fontWeight: '700' },
          }}
        >
          <Stack.Screen name="Login" component={LoginScreen} options={{ headerShown: false }} />
          <Stack.Screen name="Main" component={MainTabs} options={{ headerShown: false }} />
          <Stack.Screen name="NoteDetail" component={NoteDetailScreen} options={{ title: 'Note' }} />
          <Stack.Screen name="NoteEdit" component={NoteEditScreen} options={{ title: 'Edit Note' }} />
        </Stack.Navigator>
      </NavigationContainer>
    </SafeAreaProvider>
  );
}
