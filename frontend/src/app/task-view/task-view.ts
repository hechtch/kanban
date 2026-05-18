import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  ElementRef,
  HostListener,
  inject,
  signal,
  ViewChild,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs';

import { COLUMN_STATUSES, Project, Status, Task } from '../models';
import { TaskStore } from '../task-store';
import { renderMarkdown } from './markdown';

@Component({
  selector: 'app-task-view',
  standalone: true,
  imports: [FormsModule, RouterLink],
  templateUrl: './task-view.html',
  styleUrl: './task-view.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TaskView {
  protected store = inject(TaskStore);
  private route = inject(ActivatedRoute);
  private router = inject(Router);
  private sanitizer = inject(DomSanitizer);

  readonly statuses: Status[] = [...COLUMN_STATUSES, 'backlog'];
  readonly priorities = [0, 1, 2, 3] as const;

  private readonly idSig = toSignal(
    this.route.paramMap.pipe(map(p => Number(p.get('id')))),
    { initialValue: NaN },
  );

  readonly task = computed<Task | null>(() => {
    const id = this.idSig();
    if (!Number.isFinite(id)) return null;
    return this.store.tasks().find(t => t.id === id) ?? null;
  });

  readonly project = computed<Project | null>(() => {
    const t = this.task();
    if (!t || t.project_id == null) return null;
    return this.store.projects().find(p => p.id === t.project_id) ?? null;
  });

  // ─── edit state ─────────────────────────────────────────────────────
  readonly editingBody = signal(false);
  readonly editingTitle = signal(false);
  readonly draftBody = signal('');
  readonly draftTitle = signal('');
  readonly tagsDraft = signal('');

  readonly renderedBody = computed<SafeHtml>(() => {
    const t = this.task();
    const src = t?.body?.trim() ? t.body : '';
    return this.sanitizer.bypassSecurityTrustHtml(renderMarkdown(src));
  });

  @ViewChild('bodyEditor') bodyEditor?: ElementRef<HTMLTextAreaElement>;
  @ViewChild('titleEditor') titleEditor?: ElementRef<HTMLInputElement>;

  constructor() {
    // Keep tagsDraft synced with task when not actively editing (no tags edit
    // mode — they commit on blur/enter).
    effect(() => {
      const t = this.task();
      if (t) this.tagsDraft.set(t.tags.join(', '));
    });
  }

  // ─── title ──────────────────────────────────────────────────────────
  startTitleEdit(): void {
    const t = this.task();
    if (!t) return;
    this.draftTitle.set(t.title);
    this.editingTitle.set(true);
    queueMicrotask(() => this.titleEditor?.nativeElement.focus());
  }

  async commitTitle(): Promise<void> {
    const t = this.task();
    if (!t) return;
    const title = this.draftTitle().trim();
    this.editingTitle.set(false);
    if (!title || title === t.title) return;
    await this.store.patch(t.id, { title });
  }

  cancelTitle(): void { this.editingTitle.set(false); }

  // ─── body ───────────────────────────────────────────────────────────
  startBodyEdit(): void {
    const t = this.task();
    if (!t) return;
    this.draftBody.set(t.body ?? '');
    this.editingBody.set(true);
    queueMicrotask(() => {
      const el = this.bodyEditor?.nativeElement;
      el?.focus();
      el?.setSelectionRange(el.value.length, el.value.length);
    });
  }

  async commitBody(): Promise<void> {
    const t = this.task();
    if (!t) return;
    const body = this.draftBody();
    this.editingBody.set(false);
    if (body === (t.body ?? '')) return;
    await this.store.patch(t.id, { body });
  }

  cancelBody(): void { this.editingBody.set(false); }

  onBodyKeydown(event: KeyboardEvent): void {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault();
      this.commitBody();
    } else if (event.key === 'Escape') {
      event.preventDefault();
      this.cancelBody();
    }
  }

  // ─── metadata ───────────────────────────────────────────────────────
  async setStatus(status: Status): Promise<void> {
    const t = this.task();
    if (!t || t.status === status) return;
    await this.store.patch(t.id, { status });
  }

  async setPriority(priority: number): Promise<void> {
    const t = this.task();
    if (!t) return;
    await this.store.patch(t.id, { priority: priority as Task['priority'] });
  }

  async setDueText(due: string): Promise<void> {
    const t = this.task();
    if (!t || due === t.due_text) return;
    await this.store.patch(t.id, { due_text: due });
  }

  async setProject(value: string): Promise<void> {
    const t = this.task();
    if (!t) return;
    const project_id = value === '' ? null : Number(value);
    if (project_id === t.project_id) return;
    await this.store.patch(t.id, { project_id });
  }

  async commitTags(): Promise<void> {
    const t = this.task();
    if (!t) return;
    const tags = this.tagsDraft()
      .split(',')
      .map(s => s.trim())
      .filter(Boolean);
    if (tags.length === t.tags.length && tags.every((tag, i) => tag === t.tags[i])) {
      return;
    }
    await this.store.patch(t.id, { tags });
  }

  // ─── actions ────────────────────────────────────────────────────────
  async remove(): Promise<void> {
    const t = this.task();
    if (!t) return;
    if (!confirm(`Delete "${t.title}"?`)) return;
    await this.store.remove(t.id);
    this.router.navigate(['/board']);
  }

  prioLabel(p: number): string {
    return ['none', '! low', '!! med', '!!! high'][p] ?? '';
  }

  @HostListener('document:keydown', ['$event'])
  onKey(event: KeyboardEvent): void {
    if (this.editingBody() || this.editingTitle()) return;
    const target = event.target as HTMLElement;
    if (target?.matches?.('input, textarea, select')) return;
    if (event.key === 'e') {
      event.preventDefault();
      this.startBodyEdit();
    } else if (event.key === 'Escape') {
      this.router.navigate(['/board']);
    }
  }
}
