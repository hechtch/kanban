import { renderMarkdown, toggleCheckbox } from './markdown';

describe('renderMarkdown', () => {
  it('numbers task-list checkboxes in document order and leaves them enabled', () => {
    const html = renderMarkdown('- [ ] one\n- [x] two\n  - [ ] nested');
    const boxes = [...html.matchAll(/<input[^>]*>/g)].map(m => m[0]);
    expect(boxes.length).toBe(3);
    expect(boxes[0]).toContain('data-check="0"');
    expect(boxes[0]).not.toContain('checked');
    expect(boxes[1]).toContain('data-check="1"');
    expect(boxes[1]).toContain('checked');
    expect(boxes[2]).toContain('data-check="2"');
    expect(html).not.toContain('disabled');
  });

  it('restarts numbering on each render', () => {
    renderMarkdown('- [ ] a\n- [ ] b');
    expect(renderMarkdown('- [ ] c')).toContain('data-check="0"');
  });
});

describe('toggleCheckbox', () => {
  it('checks an unchecked item and unchecks a checked one', () => {
    const src = '- [ ] one\n- [x] two';
    expect(toggleCheckbox(src, 0)).toBe('- [x] one\n- [x] two');
    expect(toggleCheckbox(src, 1)).toBe('- [ ] one\n- [ ] two');
  });

  it('counts nested and ordered items in document order', () => {
    const src = '- [ ] a\n  - [ ] b\n1. [X] c';
    expect(toggleCheckbox(src, 1)).toBe('- [ ] a\n  - [x] b\n1. [X] c');
    expect(toggleCheckbox(src, 2)).toBe('- [ ] a\n  - [ ] b\n1. [ ] c');
  });

  it('skips task syntax inside fenced code and non-task lines', () => {
    const src = 'text [ ] not a task\n```\n- [ ] in code\n```\n- [ ] real';
    expect(toggleCheckbox(src, 0)).toBe('text [ ] not a task\n```\n- [ ] in code\n```\n- [x] real');
  });

  it('returns null when the index is out of range', () => {
    expect(toggleCheckbox('- [ ] only', 3)).toBeNull();
    expect(toggleCheckbox('no boxes', 0)).toBeNull();
  });
});
