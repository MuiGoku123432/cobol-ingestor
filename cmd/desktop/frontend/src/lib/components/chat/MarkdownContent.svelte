<script lang="ts">
  import { Marked } from 'marked';
  import hljs from 'highlight.js/lib/core';
  import java from 'highlight.js/lib/languages/java';
  import go from 'highlight.js/lib/languages/go';
  import python from 'highlight.js/lib/languages/python';
  import typescript from 'highlight.js/lib/languages/typescript';
  import javascript from 'highlight.js/lib/languages/javascript';
  import sql from 'highlight.js/lib/languages/sql';
  import json from 'highlight.js/lib/languages/json';
  import xml from 'highlight.js/lib/languages/xml';
  import bash from 'highlight.js/lib/languages/bash';
  import csharp from 'highlight.js/lib/languages/csharp';
  import kotlin from 'highlight.js/lib/languages/kotlin';
  import yaml from 'highlight.js/lib/languages/yaml';
  import 'highlight.js/styles/github-dark.css';

  hljs.registerLanguage('java', java);
  hljs.registerLanguage('go', go);
  hljs.registerLanguage('python', python);
  hljs.registerLanguage('typescript', typescript);
  hljs.registerLanguage('javascript', javascript);
  hljs.registerLanguage('sql', sql);
  hljs.registerLanguage('json', json);
  hljs.registerLanguage('xml', xml);
  hljs.registerLanguage('bash', bash);
  hljs.registerLanguage('csharp', csharp);
  hljs.registerLanguage('kotlin', kotlin);
  hljs.registerLanguage('yaml', yaml);

  const marked = new Marked({
    gfm: true,
    breaks: true,
  });

  const renderer = new marked.Renderer();
  renderer.code = function ({ text, lang }: { text: string; lang?: string }) {
    const language = lang && hljs.getLanguage(lang) ? lang : null;
    const highlighted = language
      ? hljs.highlight(text, { language }).value
      : hljs.highlightAuto(text).value;
    return `<pre><code class="hljs${language ? ` language-${language}` : ''}">${highlighted}</code></pre>`;
  };
  marked.use({ renderer });

  interface Props {
    content: string;
  }

  let { content }: Props = $props();

  let html = $derived(marked.parse(content) as string);
</script>

<div class="markdown-content">
  {@html html}
</div>

<style>
  .markdown-content {
    color: #c9d1d9;
    font-size: 13px;
    line-height: 1.6;
    word-wrap: break-word;
  }

  .markdown-content :global(h1),
  .markdown-content :global(h2),
  .markdown-content :global(h3),
  .markdown-content :global(h4) {
    color: #e1e4e8;
    margin: 16px 0 8px;
    line-height: 1.3;
  }

  .markdown-content :global(h1) {
    font-size: 1.4em;
    border-bottom: 1px solid #21262d;
    padding-bottom: 6px;
  }

  .markdown-content :global(h2) {
    font-size: 1.2em;
    border-bottom: 1px solid #21262d;
    padding-bottom: 4px;
  }

  .markdown-content :global(h3) {
    font-size: 1.05em;
  }

  .markdown-content :global(h4) {
    font-size: 1em;
    color: #8b949e;
  }

  .markdown-content :global(p) {
    margin: 8px 0;
  }

  .markdown-content :global(pre) {
    background: #161b22;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 12px;
    overflow-x: auto;
    margin: 8px 0;
  }

  .markdown-content :global(pre code) {
    background: none;
    padding: 0;
    font-size: 12px;
    font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
    line-height: 1.5;
  }

  .markdown-content :global(code) {
    background: #1c2128;
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 0.9em;
    color: #e1e4e8;
    font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
  }

  .markdown-content :global(ul),
  .markdown-content :global(ol) {
    padding-left: 24px;
    margin: 8px 0;
    color: #c9d1d9;
  }

  .markdown-content :global(li) {
    margin: 4px 0;
  }

  .markdown-content :global(table) {
    border-collapse: collapse;
    width: 100%;
    margin: 8px 0;
    font-size: 12px;
  }

  .markdown-content :global(th),
  .markdown-content :global(td) {
    border: 1px solid #21262d;
    padding: 6px 12px;
    text-align: left;
  }

  .markdown-content :global(th) {
    background: #161b22;
    color: #e1e4e8;
    font-weight: 600;
  }

  .markdown-content :global(tr:nth-child(even)) {
    background: #0d1117;
  }

  .markdown-content :global(a) {
    color: #58a6ff;
    text-decoration: none;
  }

  .markdown-content :global(a:hover) {
    text-decoration: underline;
  }

  .markdown-content :global(blockquote) {
    border-left: 3px solid #30363d;
    padding: 4px 12px;
    margin: 8px 0;
    color: #8b949e;
  }

  .markdown-content :global(strong) {
    color: #e1e4e8;
    font-weight: 600;
  }

  .markdown-content :global(em) {
    font-style: italic;
  }

  .markdown-content :global(hr) {
    border: none;
    border-top: 1px solid #21262d;
    margin: 16px 0;
  }

  .markdown-content :global(> :first-child) {
    margin-top: 0;
  }

  .markdown-content :global(> :last-child) {
    margin-bottom: 0;
  }
</style>
