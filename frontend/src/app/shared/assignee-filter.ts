// The you/Claude split, as a compact segmented control pinned at the top of
// the sidebar. Deliberately not a list like Projects and Tags: assignee is a
// fixed two-value axis that never grows, and it's the most-flipped filter in
// a board whose whole premise is agent-tracked plans — so it shouldn't be
// queued below two lists that do grow.

import { Component, inject, output } from '@angular/core';

import { Assignee, ASSIGNEE_LABEL, ASSIGNEES } from '../models';
import { TaskStore } from '../task-store';

@Component({
  selector: 'app-assignee-filter',
  standalone: true,
  template: `
    <div class="seg" role="group" aria-label="Filter by assignee">
      <button
        type="button"
        [class.on]="store.assigneeFilter() === undefined"
        [attr.aria-pressed]="store.assigneeFilter() === undefined"
        [title]="'Everyone — ' + total() + ' tasks'"
        (click)="pick(undefined)"
      >All</button>

      @for (who of assignees; track who) {
        <button
          type="button"
          [class.on]="store.assigneeFilter() === who"
          [attr.aria-pressed]="store.assigneeFilter() === who"
          [title]="hint(who)"
          (click)="pick(who)"
        >
          @if (who === 'claude') {
            <svg viewBox="0 0 16 16" width="11" height="11" aria-hidden="true">
              <path d="M8 1 Q8.4 6 13 8 Q8.4 10 8 15 Q7.6 10 3 8 Q7.6 6 8 1Z" fill="currentColor" />
            </svg>
          } @else {
            <svg viewBox="0 0 16 16" width="11" height="11" aria-hidden="true">
              <circle cx="8" cy="5.2" r="2.4" fill="none" stroke="currentColor" stroke-width="1.6" />
              <path d="M2.8 14.2 Q2.8 9.2 8 9.2 Q13.2 9.2 13.2 14.2"
                    fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            </svg>
          }
          <span>{{ label(who) }}</span>
        </button>
      }
    </div>
  `,
  styles: `
    :host { display: block; margin-bottom: 0.75rem; }
    .seg {
      display: flex;
      gap: 1px;
      padding: 2px;
      border: 1px solid var(--hairline);
      border-radius: 999px;
    }
    button {
      flex: 1;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 0.2rem;
      min-width: 0;
      padding: 0.2rem 0.1rem;
      font: inherit;
      font-size: 0.75rem;
      line-height: 1.4;
      color: var(--ink-2);
      background: transparent;
      border: none;
      border-radius: 999px;
      cursor: pointer;
    }
    button:hover { background: rgba(31, 36, 48, 0.06); color: var(--ink); }
    button.on { background: var(--accent); color: #fff; }
    button:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
    span { overflow: hidden; text-overflow: ellipsis; }
  `,
})
export class AssigneeFilter {
  protected store = inject(TaskStore);
  protected readonly assignees = ASSIGNEES;

  /** Fires on any pick so the shell can leave Search / close an open ticket. */
  readonly picked = output<void>();

  protected label(who: Assignee): string {
    return ASSIGNEE_LABEL[who];
  }

  /** Counts live in the tooltip — three labelled segments plus three numbers
   *  is more than a 14rem sidebar can carry legibly. */
  protected hint(who: Assignee): string {
    const n = this.store.assigneeCounts()[who];
    return `${ASSIGNEE_LABEL[who]} — ${n} task${n === 1 ? '' : 's'}`;
  }

  protected total(): number {
    const c = this.store.assigneeCounts();
    return c.me + c.claude;
  }

  protected pick(who: Assignee | undefined): void {
    this.store.setAssigneeFilter(who);
    this.picked.emit();
  }
}
