<script lang="ts">
  let {
    value = $bindable(''),
    placeholder = 'Search...',
    onSearch,
    debounceMs = 300,
  }: {
    value?: string;
    placeholder?: string;
    onSearch?: (query: string) => void;
    debounceMs?: number;
  } = $props();

  let timer: ReturnType<typeof setTimeout>;

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    value = target.value;
    clearTimeout(timer);
    timer = setTimeout(() => {
      onSearch?.(value);
    }, debounceMs);
  }

  function handleClear() {
    value = '';
    onSearch?.('');
  }
</script>

<div class="search-bar">
  <span class="search-icon">&#128269;</span>
  <input
    type="text"
    {placeholder}
    value={value}
    oninput={handleInput}
  />
  {#if value}
    <button class="clear-btn" onclick={handleClear}>&#10005;</button>
  {/if}
</div>

<style>
  .search-bar {
    display: flex;
    align-items: center;
    background: #0d1117;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 0 10px;
    gap: 8px;
  }

  .search-icon {
    font-size: 13px;
    color: #8b949e;
  }

  input {
    flex: 1;
    background: none;
    border: none;
    color: #e1e4e8;
    font-size: 13px;
    padding: 7px 0;
    outline: none;
  }

  input::placeholder {
    color: #484f58;
  }

  .clear-btn {
    background: none;
    border: none;
    color: #8b949e;
    cursor: pointer;
    font-size: 12px;
    padding: 2px;
  }

  .clear-btn:hover {
    color: #e1e4e8;
  }
</style>
