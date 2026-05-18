import { Component, EventEmitter, HostBinding, HostListener, Input, Output, inject } from '@angular/core';
import { Router } from '@angular/router';

import { Project, Task } from '../models';

@Component({
  selector: 'app-card',
  standalone: true,
  template: `
    <div class="title-row">
      @if (task.priority > 0) {
        <span class="prio" [attr.data-prio]="task.priority" [attr.title]="prioLabel()">
          {{ '!'.repeat(task.priority) }}
        </span>
      }
      <p class="title">{{ task.title }}</p>
      <button
        type="button"
        class="menu-btn"
        aria-label="Card actions"
        aria-haspopup="true"
        (click)="$event.stopPropagation(); menu.emit($event)"
      >⋯</button>
    </div>

    @if (task.tags.length || task.due_text || project) {
      <div class="meta">
        @if (project) {
          <span class="project">
            <span class="dot" [style.background]="project.color"></span>
            {{ project.name }}
          </span>
        }
        @if (task.due_text) {
          <span class="due">{{ task.due_text }}</span>
        }
        @for (tag of task.tags; track tag) {
          <span class="tag">#{{ tag }}</span>
        }
      </div>
    }
  `,
  styleUrl: './card.css',
})
export class Card {
  private router = inject(Router);

  @Input({ required: true }) task!: Task;
  @Input() project: Project | null = null;
  @Output() menu = new EventEmitter<MouseEvent>();

  @HostBinding('attr.tabindex') tabindex = 0;
  @HostBinding('attr.role') role = 'listitem';
  @HostBinding('attr.aria-label') get ariaLabel() {
    return `${this.task.title}, ${this.task.status}`;
  }

  @HostListener('click', ['$event'])
  onClick(event: MouseEvent): void {
    const target = event.target as HTMLElement;
    if (target.closest('button, a, input, textarea, select')) return;
    this.router.navigate(['/task', this.task.id]);
  }

  prioLabel(): string {
    return ['none', 'low', 'medium', 'high'][this.task.priority] ?? '';
  }
}
