// Small chip that names the active sidebar project filter and offers a
// one-click way out. Lives above the board and the list so the filter is
// visible even when the project sidebar is collapsed — a persisted filter
// with no on-screen cue is how "where did my tasks go?" happens.

import { Component, inject } from '@angular/core';

import { Assignee, ASSIGNEE_LABEL } from '../models';
import { TaskStore } from '../task-store';

@Component({
  selector: 'app-filter-bar',
  standalone: true,
  template: `
    @if (store.hasFilter()) {
      <div class="filter-bar" role="status">
        <span class="label">showing</span>

        @for (p of store.filterProjects(); track p.id) {
          <span class="pill">
            <span class="dot" [style.background]="p.color"></span>
            <span class="name">{{ p.name }}</span>
            <button
              type="button"
              class="clear"
              (click)="store.toggleProjectInFilter(p.id)"
              [attr.aria-label]="'Remove ' + p.name + ' from filter'"
              [title]="'Remove ' + p.name"
            >×</button>
          </span>
        }
        @if (store.inboxSelected()) {
          <span class="pill">
            <span class="name">Inbox</span>
            <button
              type="button"
              class="clear"
              (click)="store.toggleProjectInFilter(null)"
              aria-label="Remove Inbox from filter"
              title="Remove Inbox"
            >×</button>
          </span>
        }

        @if (store.tagFilter(); as tag) {
          <span class="pill">
            <span class="name"><span class="hash">#</span>{{ tag }}</span>
            <button
              type="button"
              class="clear"
              (click)="store.setTagFilter(undefined)"
              aria-label="Clear tag filter"
              title="Clear tag filter"
            >×</button>
          </span>
        }

        @if (store.assigneeFilter(); as who) {
          <span class="pill">
            <span class="name">{{ label(who) }}</span>
            <button
              type="button"
              class="clear"
              (click)="store.setAssigneeFilter(undefined)"
              aria-label="Clear assignee filter"
              title="Clear assignee filter"
            >×</button>
          </span>
        }

        @if (activeCount() > 1) {
          <button type="button" class="clear-all" (click)="store.clearFilters()">show all</button>
        }
      </div>
    }
  `,
  styles: `
    :host {
      display: block;
      padding: 0.75rem 0.75rem 0;
    }
    :host:empty { display: none; }
    .filter-bar {
      display: inline-flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 0.4rem;
      font-size: 0.85rem;
      color: var(--ink);
    }
    .label {
      font-family: var(--font-hand);
      color: var(--muted);
      margin-right: 0.1rem;
    }
    .pill {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.2rem 0.3rem 0.2rem 0.65rem;
      border: 1px solid rgba(212, 101, 74, 0.35);
      background: rgba(212, 101, 74, 0.08);
      border-radius: 999px;
    }
    .dot {
      width: 0.6rem;
      height: 0.6rem;
      border-radius: 50%;
      display: inline-block;
    }
    .name { font-weight: 500; }
    .hash { color: var(--muted); font-weight: 400; }
    .clear, .clear-all {
      font: inherit;
      color: var(--accent);
      background: transparent;
      border: none;
      border-radius: 999px;
      cursor: pointer;
    }
    .clear {
      font-size: 0.95rem;
      line-height: 1;
      padding: 0.1rem 0.4rem;
    }
    .clear-all {
      font-size: 0.8rem;
      padding: 0.2rem 0.5rem;
      text-decoration: underline dotted;
      text-underline-offset: 3px;
    }
    .clear:hover, .clear-all:hover { background: rgba(212, 101, 74, 0.15); }
    .clear:focus-visible, .clear-all:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
  `,
})
export class FilterBar {
  protected store = inject(TaskStore);

  protected label(who: Assignee): string {
    return ASSIGNEE_LABEL[who];
  }

  /** "show all" only earns its place once there's more than one thing to clear. */
  protected activeCount(): number {
    return (
      this.store.projectFilter().size +
      (this.store.tagFilter() !== undefined ? 1 : 0) +
      (this.store.assigneeFilter() !== undefined ? 1 : 0)
    );
  }
}
