import DOMPurify from 'dompurify';
import { marked } from 'marked';

marked.setOptions({ gfm: true, breaks: true });

/**
 * Task-list checkboxes get a stable index (document order) so a click in
 * the rendered view can be traced back to the matching `[ ]` / `[x]` in
 * the source — see `toggleCheckbox`. marked's default emits them
 * `disabled`; ours are live.
 */
let checkboxIndex = 0;
marked.use({
  renderer: {
    checkbox({ checked }: { checked: boolean }): string {
      const i = checkboxIndex++;
      return `<input type="checkbox" class="md-check" data-check="${i}"${checked ? ' checked=""' : ''}> `;
    },
  },
});

export function renderMarkdown(src: string): string {
  checkboxIndex = 0;
  const raw = marked.parse(src ?? '', { async: false }) as string;
  return DOMPurify.sanitize(raw, { USE_PROFILES: { html: true } });
}

/** A GFM task-list item: list marker, then `[ ]` / `[x]`, then space or EOL. */
const TASK_ITEM = /^(\s*(?:[-*+]|\d+[.)])\s+\[)([ xX])(\](?:\s|$))/;
const FENCE = /^\s{0,3}(`{3,}|~{3,})/;

/**
 * Flip the `index`-th task-list checkbox (document order, matching what
 * `renderMarkdown` numbered) in `src`. Lines inside fenced code blocks are
 * skipped, since marked doesn't render those as checkboxes either.
 * Returns the new source, or `null` if there's no such checkbox.
 */
export function toggleCheckbox(src: string, index: number): string | null {
  const lines = src.split('\n');
  let fence: string | null = null;
  let seen = 0;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const f = FENCE.exec(line);
    if (f) {
      if (fence === null) fence = f[1][0];
      else if (f[1][0] === fence) fence = null;
      continue;
    }
    if (fence !== null) continue;
    const m = TASK_ITEM.exec(line);
    if (!m) continue;
    if (seen++ === index) {
      const next = m[2] === ' ' ? 'x' : ' ';
      lines[i] = m[1] + next + m[3] + line.slice(m[0].length);
      return lines.join('\n');
    }
  }
  return null;
}
