import {
  AfterViewInit,
  Component,
  ElementRef,
  HostListener,
  ViewChild,
  computed,
  input,
  output,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';

import { Project } from '../models';

/** What the editor hands back: the writable fields of a project. */
export type ProjectDraft = Pick<Project, 'name' | 'color' | 'archived' | 'tags'>;

/**
 * Small modal for creating or editing a project — the only place a
 * project's name, colour, default tags, and archived flag are set from the
 * UI. Pass `project = null` for a new one.
 */
@Component({
  selector: 'app-project-editor',
  standalone: true,
  imports: [FormsModule],
  template: `
    <div class="overlay" (click)="cancel.emit()">
      <div
        class="dialog"
        role="dialog"
        aria-modal="true"
        [attr.aria-labelledby]="titleId"
        (click)="$event.stopPropagation()"
        (keydown.enter)="onEnter($event)"
      >
        <h2 [id]="titleId">{{ project() ? 'Edit project' : 'New project' }}</h2>

        <label>
          <span>Name</span>
          <input #nameInput type="text" [ngModel]="name()" (ngModelChange)="name.set($event)" name="name" placeholder="2026 Taxes" />
        </label>

        <label class="color">
          <span>Colour</span>
          <span class="swatches">
            <input type="color" [ngModel]="color()" (ngModelChange)="color.set($event)" name="color" aria-label="Project colour" />
            <code>{{ color() }}</code>
          </span>
        </label>

        <label>
          <span>Tags <em>every ticket in this project carries these</em></span>
          <input type="text" [ngModel]="tags()" (ngModelChange)="tags.set($event)" name="tags" placeholder="tax, finance" />
        </label>

        @if (project()) {
          <label class="check">
            <input type="checkbox" [ngModel]="archived()" (ngModelChange)="archived.set($event)" name="archived" />
            <span>Archived <em>hide from the sidebar and board; nothing is deleted</em></span>
          </label>
        }

        <div class="actions">
          @if (project()) {
            <button type="button" class="ghost danger" (click)="remove.emit()">Delete</button>
          }
          <span class="spacer"></span>
          <button type="button" class="ghost" (click)="cancel.emit()">Cancel</button>
          <button type="button" class="primary" (click)="commit()" [disabled]="!valid()">
            {{ project() ? 'Save' : 'Create' }}
          </button>
        </div>
      </div>
    </div>
  `,
  styleUrl: './project-editor.css',
})
export class ProjectEditor implements AfterViewInit {
  readonly project = input<Project | null>(null);
  readonly save = output<ProjectDraft>();
  readonly remove = output<void>();
  readonly cancel = output<void>();

  @ViewChild('nameInput', { static: true }) nameInput!: ElementRef<HTMLInputElement>;

  readonly titleId = `project-editor-${Math.random().toString(36).slice(2, 9)}`;

  readonly name = signal('');
  readonly color = signal('#1f2430');
  readonly tags = signal('');
  readonly archived = signal(false);
  readonly valid = computed(() => this.name().trim().length > 0);

  private previouslyFocused: HTMLElement | null = null;

  constructor() {
    // input() is available from the first change detection; seed the
    // drafts once from whichever project (if any) we were opened on.
    queueMicrotask(() => {
      const p = this.project();
      if (!p) return;
      this.name.set(p.name);
      this.color.set(p.color);
      this.tags.set(p.tags.join(', '));
      this.archived.set(p.archived);
    });
  }

  ngAfterViewInit(): void {
    this.previouslyFocused = document.activeElement as HTMLElement | null;
    setTimeout(() => this.nameInput.nativeElement.focus(), 0);
  }

  commit(): void {
    if (!this.valid()) return;
    this.save.emit({
      name: this.name().trim(),
      color: this.color(),
      archived: this.archived(),
      tags: this.tags().split(',').map(s => s.trim()).filter(Boolean),
    });
    this.previouslyFocused?.focus();
  }

  onEnter(event: Event): void {
    const target = event.target as HTMLElement | null;
    if (target?.matches('input[type="text"]')) {
      event.preventDefault();
      this.commit();
    }
  }

  @HostListener('document:keydown.escape', ['$event'])
  onEscape(event: Event): void {
    event.preventDefault();
    this.cancel.emit();
    this.previouslyFocused?.focus();
  }
}
