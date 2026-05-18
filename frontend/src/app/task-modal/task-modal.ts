import {
  AfterViewInit,
  Component,
  ElementRef,
  EventEmitter,
  HostListener,
  Input,
  OnInit,
  Output,
  ViewChild,
} from '@angular/core';
import { FormsModule } from '@angular/forms';

import { COLUMN_STATUSES, Project, Status, Task } from '../models';

@Component({
  selector: 'app-task-modal',
  standalone: true,
  imports: [FormsModule],
  template: `
    <div class="overlay" (click)="onOverlayClick($event)">
      <div
        class="dialog"
        role="dialog"
        aria-modal="true"
        [attr.aria-labelledby]="titleId"
        tabindex="-1"
        #box
        (click)="$event.stopPropagation()"
      >
        <h2 [id]="titleId">{{ heading }}</h2>

        <label [attr.for]="ids.title">Title</label>
        <input
          #firstFocus
          [id]="ids.title"
          type="text"
          [(ngModel)]="draft.title"
          name="title"
          required
        />

        <div class="row">
          <div>
            <label [attr.for]="ids.status">Status</label>
            <select [id]="ids.status" [(ngModel)]="draft.status" name="status">
              @for (s of statuses; track s) {
                <option [value]="s">{{ s }}</option>
              }
            </select>
          </div>
          <div>
            <label [attr.for]="ids.priority">Priority</label>
            <select [id]="ids.priority" [(ngModel)]="draft.priority" name="priority">
              <option [ngValue]="0">none</option>
              <option [ngValue]="1">! low</option>
              <option [ngValue]="2">!! medium</option>
              <option [ngValue]="3">!!! high</option>
            </select>
          </div>
        </div>

        <div class="row">
          <div>
            <label [attr.for]="ids.due">Due</label>
            <input
              [id]="ids.due"
              type="text"
              [(ngModel)]="draft.due_text"
              name="due_text"
              placeholder="today · fri · next wk"
            />
          </div>
          <div>
            <label [attr.for]="ids.project">Project</label>
            <select [id]="ids.project" [(ngModel)]="draft.project_id" name="project_id">
              <option [ngValue]="null">— inbox —</option>
              @for (p of projects; track p.id) {
                <option [ngValue]="p.id">{{ p.name }}</option>
              }
            </select>
          </div>
        </div>

        <label [attr.for]="ids.tags">Tags (comma-separated)</label>
        <input
          [id]="ids.tags"
          type="text"
          [ngModel]="tagsText"
          (ngModelChange)="tagsText = $event"
          name="tags"
          placeholder="ping, urgent"
        />

        <div class="actions">
          <button type="button" class="ghost" (click)="cancel.emit()">Cancel</button>
          <button type="button" class="primary" (click)="submit()" [disabled]="!draft.title.trim()">
            {{ mode === 'edit' ? 'Save' : 'Create' }}
          </button>
        </div>
      </div>
    </div>
  `,
  styleUrl: './task-modal.css',
})
export class TaskModal implements OnInit, AfterViewInit {
  @Input({ required: true }) mode!: 'new' | 'edit';
  @Input() task: Task | null = null;
  @Input() projects: Project[] = [];
  @Input() initialStatus: Status = 'todo';
  /** When `mode === 'new'`, pre-populate from a parsed draft. */
  @Input() prefill: Partial<Task> | null = null;

  @Output() save = new EventEmitter<Partial<Task>>();
  @Output() cancel = new EventEmitter<void>();

  @ViewChild('box', { static: true }) box!: ElementRef<HTMLDivElement>;
  @ViewChild('firstFocus', { static: true }) firstFocus!: ElementRef<HTMLInputElement>;

  readonly statuses = [...COLUMN_STATUSES, 'backlog' as Status];
  readonly titleId = `task-modal-${Math.random().toString(36).slice(2, 9)}`;
  readonly ids = {
    title: `${this.titleId}-title`,
    status: `${this.titleId}-status`,
    priority: `${this.titleId}-priority`,
    due: `${this.titleId}-due`,
    project: `${this.titleId}-project`,
    tags: `${this.titleId}-tags`,
  };

  draft: {
    title: string;
    status: Status;
    priority: 0 | 1 | 2 | 3;
    due_text: string;
    project_id: number | null;
  } = { title: '', status: 'todo', priority: 0, due_text: '', project_id: null };

  tagsText = '';

  private previouslyFocused: HTMLElement | null = null;

  get heading(): string {
    return this.mode === 'edit' ? 'Edit task' : 'New task';
  }

  ngOnInit(): void {
    if (this.mode === 'edit' && this.task) {
      this.draft = {
        title: this.task.title,
        status: this.task.status,
        priority: this.task.priority,
        due_text: this.task.due_text,
        project_id: this.task.project_id,
      };
      this.tagsText = this.task.tags.join(', ');
    } else {
      this.draft.status = this.initialStatus;
      if (this.prefill) {
        if (this.prefill.title != null) this.draft.title = this.prefill.title;
        if (this.prefill.priority != null) this.draft.priority = this.prefill.priority;
        if (this.prefill.due_text != null) this.draft.due_text = this.prefill.due_text;
        if (this.prefill.project_id != null) this.draft.project_id = this.prefill.project_id;
        if (this.prefill.tags != null) this.tagsText = this.prefill.tags.join(', ');
      }
    }
  }

  ngAfterViewInit(): void {
    this.previouslyFocused = document.activeElement as HTMLElement | null;
    setTimeout(() => this.firstFocus.nativeElement.focus(), 0);
  }

  submit(): void {
    if (!this.draft.title.trim()) return;
    const tags = this.tagsText
      .split(',')
      .map(s => s.trim())
      .filter(Boolean);
    this.save.emit({ ...this.draft, tags });
    this.restoreFocus();
  }

  onOverlayClick(_: MouseEvent): void {
    this.cancel.emit();
  }

  @HostListener('document:keydown.escape', ['$event'])
  onEscape(event: Event): void {
    event.preventDefault();
    this.cancel.emit();
    this.restoreFocus();
  }

  @HostListener('keydown.tab', ['$event'])
  @HostListener('keydown.shift.tab', ['$event'])
  trapFocus(event: Event): void {
    const ke = event as KeyboardEvent;
    const focusables = this.box.nativeElement.querySelectorAll<HTMLElement>(
      'input, select, textarea, button, [tabindex]:not([tabindex="-1"])'
    );
    if (!focusables.length) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const active = document.activeElement as HTMLElement;
    if (ke.shiftKey && active === first) {
      ke.preventDefault();
      last.focus();
    } else if (!ke.shiftKey && active === last) {
      ke.preventDefault();
      first.focus();
    }
  }

  private restoreFocus(): void {
    this.previouslyFocused?.focus();
  }
}
