<script lang="ts">
  let {
    text,
    variant = 'default',
  }: {
    text: string;
    variant?: 'default' | 'success' | 'warning' | 'danger' | 'info' | 'leaf' | 'middle' | 'root' | 'S' | 'M' | 'L' | 'XL';
  } = $props();

  const colorMap: Record<string, string> = {
    default: '#30363d',
    success: '#238636',
    warning: '#d29922',
    danger: '#f85149',
    info: '#58a6ff',
    leaf: '#238636',
    middle: '#d29922',
    root: '#f85149',
    S: '#238636',
    M: '#58a6ff',
    L: '#d29922',
    XL: '#f85149',
  };
</script>

<span class="badge" style="background: {colorMap[variant] || colorMap.default}">
  {text}
</span>

<style>
  .badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 10px;
    font-size: 11px;
    font-weight: 600;
    color: #fff;
    white-space: nowrap;
  }
</style>
