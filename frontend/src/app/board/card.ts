import { Component, EventEmitter, HostBinding, HostListener, Input, Output, inject } from '@angular/core';
import { TicketNav } from '../shared/ticket-nav';

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

    @if (task.git_branch) {
      <div class="branch" title="git branch">
        <svg class="branch-icon" viewBox="0 0 16 16" aria-hidden="true">
          <circle cx="4" cy="3" r="1.5" fill="none" stroke="currentColor" stroke-width="1.3" />
          <circle cx="4" cy="13" r="1.5" fill="none" stroke="currentColor" stroke-width="1.3" />
          <circle cx="12" cy="3" r="1.5" fill="none" stroke="currentColor" stroke-width="1.3" />
          <path d="M4 4.6 V11.4 M4 8 Q4 5 8 5 H10.5"
                fill="none" stroke="currentColor" stroke-width="1.3"
                stroke-linecap="round" />
        </svg>
        <span>{{ task.git_branch }}</span>
      </div>
    }

    @if (task.model || task.effort) {
      <div class="model" title="suggested model / effort">
        <svg class="model-icon" viewBox="0 0 16 16" aria-hidden="true">
          <rect x="4" y="4" width="8" height="8" rx="1.5" fill="none"
                stroke="currentColor" stroke-width="1.3" />
          <path d="M6 1.5V4 M10 1.5V4 M6 12v2.5 M10 12v2.5 M1.5 6H4 M1.5 10H4 M12 6h2.5 M12 10h2.5"
                stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
        </svg>
        <span>{{ modelLabel }}</span>
      </div>
    }

    <div class="actor" [class.claude]="isClaude"
         [attr.title]="isClaude ? 'Claude-owned plan' : 'User-entered task'">
      @if (isClaude) {
        <svg class="actor-icon" viewBox="0 0 16 16" aria-hidden="true">
          <path d="M8 1 Q8.4 6 13 8 Q8.4 10 8 15 Q7.6 10 3 8 Q7.6 6 8 1Z"
                fill="currentColor" />
        </svg>
        <span>Claude</span>
      } @else {
        <svg class="actor-icon" viewBox="0 0 16 16" aria-hidden="true">
          <circle cx="8" cy="5.2" r="2.4" fill="none"
                  stroke="currentColor" stroke-width="1.3" />
          <path d="M2.8 14.2 Q2.8 9.2 8 9.2 Q13.2 9.2 13.2 14.2"
                fill="none" stroke="currentColor"
                stroke-width="1.3" stroke-linecap="round" />
        </svg>
        <span>you</span>
      }
    </div>
  `,
  styleUrl: './card.css',
})
export class Card {
  private ticketNav = inject(TicketNav);

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
    this.ticketNav.open(this.task.id);
  }

  prioLabel(): string {
    return ['none', 'low', 'medium', 'high'][this.task.priority] ?? '';
  }

  get isClaude(): boolean {
    return this.task.plan_slug != null;
  }

  /** `fable / xhigh`, or whichever half is set. */
  get modelLabel(): string {
    return [this.task.model, this.task.effort].filter(Boolean).join(' / ');
  }
}
