import React from 'react';
import { TouchableOpacity, Text, StyleSheet } from 'react-native';

interface Props {
  label: string;
  active?: boolean;
  onPress?: () => void;
  variant?: 'tag' | 'category';
}

export function TagChip({ label, active, onPress, variant = 'tag' }: Props) {
  const isCategory = variant === 'category';
  return (
    <TouchableOpacity
      style={[styles.chip, isCategory && styles.catChip, active && styles.activeChip]}
      onPress={onPress}
      disabled={!onPress}
    >
      <Text style={[styles.text, active && styles.activeText]}>
        {isCategory ? '📁 ' : '#'}{label}
      </Text>
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  chip: {
    borderRadius: 14,
    paddingHorizontal: 10,
    paddingVertical: 4,
    backgroundColor: '#e0e7ff',
    marginRight: 6,
    marginBottom: 4,
  },
  catChip: {
    backgroundColor: '#fef3c7',
  },
  activeChip: {
    backgroundColor: '#4f46e5',
  },
  text: {
    fontSize: 12,
    color: '#4338ca',
    fontWeight: '500',
  },
  activeText: {
    color: '#fff',
  },
});
