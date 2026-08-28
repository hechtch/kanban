import { Component, HostListener, computed, inject, signal } from '@angular/core';
import { ActivatedRoute, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs';

import { Assignee, ASSIGNEE_LABEL, ASSIGNEES, Project, Task } from './models';
import { TaskStore } from './task-store';
import { Capture } from './task-modal/capture';
import { DashboardNav } from './shared/dashboard-nav';
import { TICKET_PARAM, TicketNav } from './shared/ticket-nav';
import { TaskView } from './task-view/task-view';
import { ProjectDraft, ProjectEditor } from './project-editor/project-editor';
import pkg from '../../package.json';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Capture, DashboardNav, TaskView, ProjectEditor],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App {
  /** Built-from version, shown beside the app name so a glance at the header
   *  tells whether the deployed bundle matches the source's package.json. */
  readonly version = pkg.version;

  private store = inject(TaskStore);
  private router = inject(Router);
  private route = inject(ActivatedRoute);
  private ticketNav = inject(TicketNav);

  /** Ticket open over the current view (`?task=<id>`), or null. */
  readonly ticketId = toSignal(
    this.route.queryParamMap.pipe(
      map(p => {
        const raw = p.get(TICKET_PARAM);
        const id = Number(raw);
        return raw && Number.isInteger(id) ? id : null;
      }),
    ),
    { initialValue: null },
  );

  // <base href> like "/apps/kanban/" means we're hosted inside the dashboard
  // reverse proxy; show a back-link to "/".
  private readonly baseHref = document.querySelector('base')?.getAttribute('href') ?? '/';
  readonly inDashboard = this.baseHref.startsWith('/apps/');

  readonly projects = this.store.activeProjects;
  readonly archivedProjects = this.store.archivedProjects;
  readonly tasks = this.store.activeTasks;
  readonly filter = this.store.projectFilter;
  /** The archived section of the sidebar is folded by default — it's the attic. */
  readonly archivedOpen = signal(false);
  /** Project editor state: `undefined` closed, `null` new project, else editing. */
  readonly editingProject = signal<Project | null | undefined>(undefined);
  /** Last project row clicked without Shift — the start of a Shift-click range. */
  private rangeAnchor: number | null | undefined = undefined;
  readonly tagFilter = this.store.tagFilter;
  readonly tags = this.store.tagCounts;
  readonly assignees = ASSIGNEES;
  readonly assigneeFilter = this.store.assigneeFilter;
  readonly assigneeCounts = this.store.assigneeCounts;

  assigneeLabel(who: Assignee): string {
    return ASSIGNEE_LABEL[who];
  }
  readonly captureOpen = signal(false);
  readonly navMenuOpen = signal(false);
  readonly sidebarOpen = signal(this.loadSidebarOpen());

  toggleNavMenu(_: MouseEvent): void {
    this.navMenuOpen.update(v => !v);
  }

  private loadSidebarOpen(): boolean {
    try {
      const v = localStorage.getItem('kanban-sidebar-open');
      return v === null ? true : v === '1';
    } catch {
      return true;
    }
  }

  toggleSidebar(): void {
    const next = !this.sidebarOpen();
    this.sidebarOpen.set(next);
    try { localStorage.setItem('kanban-sidebar-open', next ? '1' : '0'); } catch { /* ignore */ }
  }

  readonly countByProject = computed(() => {
    const counts = new Map<number | null, number>();
    for (const t of this.store.tasks()) {
      counts.set(t.project_id, (counts.get(t.project_id) ?? 0) + 1);
    }
    return counts;
  });

  countFor(id: number | null): number {
    return this.countByProject().get(id) ?? 0;
  }

  isSelected(id: number | null): boolean {
    return this.filter().has(id);
  }

  /**
   * Sidebar click on a project (or Inbox).
   *  - plain: select just this row; clicking the sole selected row clears
   *  - Ctrl/⌘: toggle this row in the selection
   *  - Shift: select the range from the last plain/Ctrl-clicked row to this
   *    one, in sidebar order (Inbox last); Ctrl+Shift adds the range
   */
  selectProject(id: number | null, event?: MouseEvent): void {
    const multi = !!(event?.ctrlKey || event?.metaKey);
    if (event?.shiftKey && this.rangeAnchor !== undefined) {
      const order = this.sidebarOrder();
      const a = order.indexOf(this.rangeAnchor);
      const b = order.indexOf(id);
      if (a >= 0 && b >= 0) {
        const range = order.slice(Math.min(a, b), Math.max(a, b) + 1);
        this.store.setProjectFilter(multi ? [...this.filter(), ...range] : range);
        this.returnToBoard();
        return;
      }
    }
    if (multi) this.store.toggleProjectInFilter(id);
    else this.store.toggleProjectFilter(id);
    this.rangeAnchor = id;
    this.returnToBoard();
  }

  selectAll(): void {
    this.store.setProjectFilter(undefined);
    this.rangeAnchor = undefined;
    this.returnToBoard();
  }

  /** Rows as they appear in the sidebar: projects, Inbox, then any unfolded archived ones. */
  private sidebarOrder(): (number | null)[] {
    const archived = this.archivedOpen() ? this.archivedProjects().map(p => p.id) : [];
    return [...this.projects().map(p => p.id), null, ...archived];
  }

  toggleArchived(): void {
    this.archivedOpen.update(v => !v);
  }

  // ─── project editor ─────────────────────────────────────────────────
  openProjectEditor(p: Project | null, event?: Event): void {
    event?.stopPropagation();
    this.editingProject.set(p);
  }

  closeProjectEditor(): void {
    this.editingProject.set(undefined);
  }

  async saveProject(draft: ProjectDraft): Promise<void> {
    const p = this.editingProject();
    this.editingProject.set(undefined);
    if (p) {
      await this.store.patchProject(p.id, draft);
    } else {
      await this.store.createProject(draft);
    }
  }

  async deleteProject(): Promise<void> {
    const p = this.editingProject();
    if (!p) return;
    const n = this.countFor(p.id);
    const tail = n ? ` Its ${n} task${n === 1 ? '' : 's'} will move to Inbox.` : '';
    if (!confirm(`Delete project "${p.name}"?${tail}`)) return;
    this.editingProject.set(undefined);
    await this.store.removeProject(p.id);
  }

  /** Tag rows work the same way: click to select, click again to clear. */
  selectTag(tag: string): void {
    this.store.toggleTagFilter(tag);
    this.returnToBoard();
  }

  /** Assignee is derived from plan ownership, so these two rows are fixed. */
  selectAssignee(who: Assignee): void {
    this.store.toggleAssigneeFilter(who);
    this.returnToBoard();
  }

  /**
   * Picking a sidebar row means "show me that project". If a ticket is open,
   * close it so the (now filtered) board behind it is what they see; if
   * they're on Search — where the sidebar filter doesn't apply — jump to
   * the Board.
   */
  private returnToBoard(): void {
    if (this.router.url.startsWith('/search')) {
      this.router.navigateByUrl('/board');
    } else if (this.ticketId() !== null) {
      this.ticketNav.close();
    }
  }

  openCapture(): void { this.captureOpen.set(true); }
  closeCapture(): void { this.captureOpen.set(false); }

  async onCaptureCreate(draft: Partial<Task>): Promise<void> {
    this.captureOpen.set(false);
    await this.store.create(this.store.withFilterDefaults(draft));
  }

  // ⌘N / Ctrl+N opens quick capture from anywhere — but not when the user is
  // typing in a form control (their own Ctrl+N reach is the browser's "new
  // window," but inside an input we shouldn't hijack the meaning either way
  // if the field is intended for prose).
  @HostListener('document:keydown', ['$event'])
  onGlobalKey(event: Event): void {
    const ke = event as KeyboardEvent;
    if ((ke.metaKey || ke.ctrlKey) && ke.key?.toLowerCase() === 'n') {
      ke.preventDefault();
      this.captureOpen.set(true);
      return;
    }
    if (ke.key === 'Escape' && this.navMenuOpen()) {
      this.navMenuOpen.set(false);
    }
    // ⌘F or `/` from anywhere: jump to the Search tab and focus its input.
    // The browser's default ⌘F (DOM find) is fine to give up — the kanban's
    // search hits the DB, not the rendered text.
    const target = ke.target as HTMLElement | null;
    const inField = !!target?.matches?.('input, textarea, select, [contenteditable="true"]');
    if ((ke.metaKey || ke.ctrlKey) && ke.key?.toLowerCase() === 'f') {
      ke.preventDefault();
      this.router.navigateByUrl('/search');
    } else if (ke.key === '/' && !inField) {
      ke.preventDefault();
      this.router.navigateByUrl('/search');
    }
  }

  @HostListener('document:click')
  onDocumentClick(): void {
    if (this.navMenuOpen()) this.navMenuOpen.set(false);
  }
}
