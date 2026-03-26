export function usePersistedState<T extends Record<string, unknown>>(page: string, defaults: T): T {
  const state: T = $state({ ...defaults });

  for (const key of Object.keys(defaults)) {
    const stored = localStorage.getItem(`cgraph:${page}:${key}`);
    if (stored !== null) {
      try {
        (state as any)[key] = JSON.parse(stored);
      } catch {
        // keep default on parse error
      }
    }
  }

  for (const key of Object.keys(defaults)) {
    $effect(() => {
      localStorage.setItem(`cgraph:${page}:${key}`, JSON.stringify((state as any)[key]));
    });
  }

  return state;
}
