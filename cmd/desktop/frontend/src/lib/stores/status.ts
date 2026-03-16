import { writable } from 'svelte/store';

export interface Neo4jStatus {
  connected: boolean;
  uri: string;
  database: string;
  error?: string;
}

export interface AuthStatus {
  ready: boolean;
  provider: string;
  model: string;
  pending: boolean;
}

export const neo4jStatus = writable<Neo4jStatus>({
  connected: false,
  uri: '',
  database: '',
});

export const authStatus = writable<AuthStatus>({
  ready: false,
  provider: '',
  model: '',
  pending: false,
});

// Refresh status from Go backend
export async function refreshNeo4jStatus() {
  try {
    // @ts-ignore - Wails bindings
    const status = await window.go.main.Neo4jService.GetStatus();
    neo4jStatus.set(status);
  } catch (e) {
    console.error('Failed to get Neo4j status:', e);
  }
}

export async function refreshAuthStatus() {
  try {
    // @ts-ignore - Wails bindings
    const status = await window.go.main.ChatService.GetAuthStatus();
    authStatus.set(status);
  } catch (e) {
    console.error('Failed to get auth status:', e);
  }
}

// Initialize on load
refreshNeo4jStatus();
refreshAuthStatus();
