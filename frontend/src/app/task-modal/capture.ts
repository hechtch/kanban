import {
  AfterViewInit,
  Component,
  ElementRef,
  EventEmitter,
  HostListener,
  Input,
  Output,
  ViewChild,
  inject,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';

import { ApiService } from '../api.service';
import { ParsedDraft, Project, Task } from '../models';

@Component({
  selector: 'app-capture',
  standalone: true,
  imports: [FormsModule],
  template: `
    <div class="overlay" (click)="cancel.emit()">
      <div
        class="dialog"
        role="dialog"
        aria-modal="true"
        [attr.aria-labelledby]="titleId"
        tabindex="-1"
        (click)="$event.stopPropagation()"
      >
        <h2 [id]="titleId">Quick capture</h2>
        <p class="hint">
          e.g. <span class="example">email landlord about the leak by friday !! &#64;admin #ping</span>
        </p>
        <input
          #input
          type="text"
          [(ngModel)]="text"
          (ngModelChange)="onTextChange($event)"
          (keydown.enter)="commit()"
          name="capture"
          aria-label="Capture text"
          placeholder="Type your task…"
        />

        @if (preview()) {
          <dl class="preview">
            <dt>Title</dt><dd>{{ preview()!.title || '—' }}</dd>
            <dt>Priority</dt>
            <dd>{{ preview()!.priority === 0 ? 'none' : '!'.repeat(preview()!.priority) }}</dd>
            <dt>Due</dt><dd>{{ preview()!.due_text || '—' }}</dd>
            <dt>Project</dt>
            <dd>
              {{ preview()!.project_name || '—' }}
              @if (preview()!.project_name && !preview()!.project_id) {
                <span class="warn">(no matching project — leaves Inbox)</span>
              }
            </dd>
            <dt>Tags</dt>
            <dd>
              @if (preview()!.tags.length) {
                @for (t of preview()!.tags; track t) {
                  <span class="tag">#{{ t }}</span>
                }
              } @else { — }
            </dd>
          </dl>
        }

        <div class="actions">
          <button type="button" class="ghost" (click)="cancel.emit()">Cancel</button>
          <button
            type="button"
            class="primary"
            (click)="commit()"
            [disabled]="!preview() || !preview()!.title.trim()"
          >Create</button>
        </div>
      </div>
    </div>
  `,
  styleUrl: './capture.css',
})
export class Capture implements AfterViewInit {
  private api = inject(ApiService);

  @Input() projects: Project[] = [];
  @Output() create = new EventEmitter<Partial<Task>>();
  @Output() cancel = new EventEmitter<void>();

  @ViewChild('input', { static: true }) inputEl!: ElementRef<HTMLInputElement>;

  readonly titleId = `capture-${Math.random().toString(36).slice(2, 9)}`;

  text = '';
  preview = signal<ParsedDraft | null>(null);

  private debounce: ReturnType<typeof setTimeout> | null = null;
  private previouslyFocused: HTMLElement | null = null;

  ngAfterViewInit(): void {
    this.previouslyFocused = document.activeElement as HTMLElement | null;
    setTimeout(() => this.inputEl.nativeElement.focus(), 0);
  }

  onTextChange(text: string): void {
    if (this.debounce) clearTimeout(this.debounce);
    if (!text.trim()) {
      this.preview.set(null);
      return;
    }
    this.debounce = setTimeout(() => this.fetchPreview(text), 150);
  }

  private fetchPreview(text: string): void {
    this.api.parseTask(text).subscribe(p => this.preview.set(p));
  }

  commit(): void {
    const p = this.preview();
    if (!p || !p.title.trim()) return;
    this.create.emit({
      title: p.title,
      priority: p.priority,
      due_text: p.due_text,
      project_id: p.project_id,
      tags: p.tags,
      status: 'todo',
    });
    this.previouslyFocused?.focus();
  }

  @HostListener('document:keydown.escape', ['$event'])
  onEscape(event: Event): void {
    event.preventDefault();
    this.cancel.emit();
    this.previouslyFocused?.focus();
  }
}
