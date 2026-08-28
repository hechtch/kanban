import {
  AfterViewInit,
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  ElementRef,
  HostListener,
  inject,
  input,
  OnDestroy,
  signal,
  ViewChild,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

import { COLUMN_STATUSES, EFFORT_OPTIONS, MODEL_OPTIONS, Project, Status, STATUS_LABEL, Task } from '../models';
import { TaskStore } from '../task-store';
import { TicketNav } from '../shared/ticket-nav';
import { confirmDelete } from '../shared/confirm-delete';
import { renderMarkdown, toggleCheckbox } from './markdown';

/**
 * The ticket, as a big modal over whatever view is underneath. Hosted by
 * `App` whenever `?task=<id>` is in the URL; closing means clearing that
 * param (see `TicketNav`), so the board/list behind it is untouched.
 */
@Component({
  selector: 'app-task-view',
  standalone: true,
  imports: [FormsModule],
  templateUrl: './task-view.html',
  styleUrl: './task-view.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TaskView implements AfterViewInit, OnDestroy {
  protected store = inject(TaskStore);
  protected nav = inject(TicketNav);
  private sanitizer = inject(DomSanitizer);

  readonly taskId = input.required<number>();

  readonly statuses: Status[] = [...COLUMN_STATUSES, 'backlog'];

  statusLabel(s: Status): string {
    return STATUS_LABEL[s];
  }
  readonly priorities = [0, 1, 2, 3] as const;
  readonly models: readonly string[] = MODEL_OPTIONS;
  readonly efforts: readonly string[] = EFFORT_OPTIONS;

  readonly task = computed<Task | null>(() => {
    const id = this.taskId();
    return this.store.tasks().find(t => t.id === id) ?? null;
  });

  /** True until the first task load lands — avoids a "not found" flash on deep links. */
  readonly loading = computed(() => this.store.tasks().length === 0);

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
  @ViewChild('box') box?: ElementRef<HTMLElement>;

  private previouslyFocused: HTMLElement | null = null;

  ngAfterViewInit(): void {
    // Take focus so Esc / `e` land here, and hand it back to the card (or
    // whatever opened us) on close.
    this.previouslyFocused = document.activeElement as HTMLElement | null;
    setTimeout(() => this.box?.nativeElement.focus(), 0);
  }

  ngOnDestroy(): void {
    this.previouslyFocused?.focus?.();
  }

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

  /**
   * Clicks inside the rendered body: a task-list checkbox flips the matching
   * `[ ]` / `[x]` in the source and saves. Default is prevented so the DOM
   * stays a pure function of the task — the re-render carries the new state
   * (and a failed PATCH rolls it back visibly).
   */
  async onBodyClick(event: MouseEvent): Promise<void> {
    const el = event.target as HTMLElement | null;
    if (!(el instanceof HTMLInputElement) || !el.classList.contains('md-check')) return;
    event.preventDefault();
    const t = this.task();
    if (!t) return;
    const body = toggleCheckbox(t.body ?? '', Number(el.dataset['check']));
    if (body === null) return;
    await this.store.patch(t.id, { body });
  }

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

  async setModel(value: string): Promise<void> {
    const t = this.task();
    if (!t || (value || null) === (t.model ?? null)) return;
    await this.store.patch(t.id, { model: value || null });
  }

  async setEffort(value: string): Promise<void> {
    const t = this.task();
    if (!t || (value || null) === (t.effort ?? null)) return;
    await this.store.patch(t.id, { effort: value || null });
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
    if (!confirmDelete(t)) return;
    await this.store.remove(t.id);
    this.nav.close();
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
      this.nav.close();
    }
  }
}
