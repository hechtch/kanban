import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, firstValueFrom } from 'rxjs';

import { ApiService } from './api.service';
import { Assignee, assigneeOf, COLUMN_STATUSES, Project, Status, Task, TaskFilter } from './models';

/**
 * One entry in the sidebar project filter: a project id, or `null` for
 * Inbox (tasks with no project). The filter is a set of these; an empty
 * set means "all".
 */
export type ProjectKey = number | null;

const FILTER_KEY = 'kanban-project-filter';
const TAG_FILTER_KEY = 'kanban-tag-filter';
const ASSIGNEE_FILTER_KEY = 'kanban-assignee-filter';

export interface TagCount {
  name: string;
  count: number;
}

@Injectable({ providedIn: 'root' })
export class TaskStore {
  private api = inject(ApiService);

  private readonly _tasks = signal<Task[]>([]);
  private readonly _projects = signal<Project[]>([]);
  private readonly _projectFilter = signal<ReadonlySet<ProjectKey>>(loadFilter());
  private readonly _tagFilter = signal<string | undefined>(loadTagFilter());
  private readonly _assigneeFilter = signal<Assignee | undefined>(loadAssigneeFilter());

  readonly tasks = this._tasks.asReadonly();
  readonly projects = this._projects.asReadonly();

  readonly activeProjects = computed(() => this._projects().filter(p => !p.archived));
  readonly archivedProjects = computed(() => this._projects().filter(p => p.archived));
  private readonly archivedIds = computed(() => new Set(this.archivedProjects().map(p => p.id)));

  /** Tasks outside archived projects — what "All" shows and counts. */
  readonly activeTasks = computed(() => {
    const archived = this.archivedIds();
    if (!archived.size) return this._tasks();
    return this._tasks().filter(t => t.project_id === null || !archived.has(t.project_id));
  });

  /**
   * `activeTasks`, plus the tasks of any archived project the sidebar
   * filter names explicitly. Archiving hides a finished project's tasks by
   * default, but picking it from the archived section still shows them.
   */
  private readonly liveTasks = computed(() => {
    const archived = this.archivedIds();
    if (!archived.size) return this._tasks();
    const f = this.projectFilter();
    return this._tasks().filter(
      t => t.project_id === null || !archived.has(t.project_id) || f.has(t.project_id),
    );
  });

  /**
   * The active sidebar project filter. Empty set = all. Stale project ids
   * (deleted since the selection was persisted) are dropped once projects
   * have loaded, so the board never silently goes blank.
   */
  readonly projectFilter = computed<ReadonlySet<ProjectKey>>(() => {
    const f = this._projectFilter();
    const projects = this._projects();
    if (!projects.length) return f;
    const live = new Set<ProjectKey>();
    for (const key of f) {
      if (key === null || projects.some(p => p.id === key)) live.add(key);
    }
    return live.size === f.size ? f : live;
  });

  /** Selected projects, in sidebar order (Inbox is `inboxSelected`, not here). */
  readonly filterProjects = computed<Project[]>(() => {
    const f = this.projectFilter();
    return this._projects().filter(p => f.has(p.id));
  });

  readonly inboxSelected = computed(() => this.projectFilter().has(null));

  /**
   * The single project the filter points at, or null when the selection is
   * empty, is Inbox, or spans several projects. Used to default new tasks —
   * with several projects selected there's no right answer, so none is picked.
   */
  readonly soleFilterProject = computed<Project | null>(() => {
    const f = this.projectFilter();
    if (f.size !== 1 || f.has(null)) return null;
    const [id] = f;
    return this._projects().find(p => p.id === id) ?? null;
  });

  /**
   * Every tag in use, with how many tasks carry it, most-used first. Derived
   * from the loaded task set rather than a `/api/tags` call so it can never
   * disagree with the cards on screen.
   */
  readonly tagCounts = computed<TagCount[]>(() => {
    const counts = new Map<string, number>();
    for (const t of this.liveTasks()) {
      for (const tag of t.tags) counts.set(tag, (counts.get(tag) ?? 0) + 1);
    }
    return [...counts.entries()]
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
  });

  /**
   * The active sidebar tag filter, or `undefined`. A tag no task carries any
   * more (last one untagged or deleted) collapses to "none" — there'd be
   * nothing to show, and no row in the sidebar to clear it from.
   */
  readonly tagFilter = computed<string | undefined>(() => {
    const tag = this._tagFilter();
    const tasks = this._tasks();
    if (tag !== undefined && tasks.length && !tasks.some(t => t.tags.includes(tag))) {
      return undefined;
    }
    return tag;
  });

  /**
   * The active assignee filter, or `undefined`. Unlike the tag filter this
   * never auto-clears: both rows are always in the sidebar, so an empty
   * result still has a visible way out.
   */
  readonly assigneeFilter = this._assigneeFilter.asReadonly();

  /** How many tasks each assignee owns — same basis as `tagCounts`. */
  readonly assigneeCounts = computed<Record<Assignee, number>>(() => {
    const out: Record<Assignee, number> = { me: 0, claude: 0 };
    for (const t of this.liveTasks()) out[assigneeOf(t)]++;
    return out;
  });

  readonly hasFilter = computed(
    () =>
      this.projectFilter().size > 0 ||
      this.tagFilter() !== undefined ||
      this.assigneeFilter() !== undefined,
  );

  /**
   * `tasks` narrowed by `projectFilter` and `tagFilter`. Board and List
   * render from this. The tag filter deliberately spans projects — that's
   * its whole point (`#finance` across Taxes and HSA, say) — and the two
   * filters AND together when both are set.
   */
  readonly visibleTasks = computed(() => {
    const f = this.projectFilter();
    const tag = this.tagFilter();
    const who = this.assigneeFilter();
    let out = this.liveTasks();
    if (f.size) out = out.filter(t => f.has(t.project_id));
    if (tag !== undefined) out = out.filter(t => t.tags.includes(tag));
    if (who !== undefined) out = out.filter(t => assigneeOf(t) === who);
    return out;
  });

  /**
   * Visible tasks bucketed by column and sorted. Deliberately built from
   * `visibleTasks`, not `tasks`: `move()` computes drop neighbours from this
   * map, and a drop index from the board is an index into what's on screen.
   */
  readonly tasksByColumn = computed(() => {
    const out = new Map<Status, Task[]>();
    for (const s of COLUMN_STATUSES) out.set(s, []);
    for (const t of this.visibleTasks()) {
      const bucket = out.get(t.status);
      if (bucket) bucket.push(t);
    }
    for (const list of out.values()) list.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id);
    return out;
  });

  constructor() {
    this.refresh();
  }

  refresh(): void {
    this.api.listProjects().subscribe(p => this._projects.set(p));
    this.refreshTasks();
  }

  private refreshTasks(): void {
    this.api.listTasks().subscribe(t => this._tasks.set(t));
  }

  // ─── projects ───────────────────────────────────────────────────────

  async createProject(input: Partial<Project>): Promise<Project> {
    const created = await firstValueFrom(this.api.createProject(input));
    this._projects.update(ps => [...ps, created]);
    return created;
  }

  /**
   * Optimistic like `patch`. A change to the project's tags changes every
   * task's effective tags, and the server is where that merge happens, so
   * the task list is re-read afterwards.
   */
  async patchProject(id: number, patch: Partial<Project>): Promise<Project> {
    const prev = this._projects();
    this._projects.set(prev.map(p => (p.id === id ? { ...p, ...patch } as Project : p)));
    try {
      const updated = await firstValueFrom(this.api.patchProject(id, patch));
      this._projects.update(ps => ps.map(p => (p.id === id ? updated : p)));
      if (patch.tags !== undefined) this.refreshTasks();
      return updated;
    } catch (e) {
      this._projects.set(prev);
      throw e;
    }
  }

  /** Tasks in the project survive and land in Inbox (the FK is ON DELETE SET NULL). */
  async removeProject(id: number): Promise<void> {
    const prevProjects = this._projects();
    const prevTasks = this._tasks();
    this._projects.set(prevProjects.filter(p => p.id !== id));
    this._tasks.set(prevTasks.map(t => (t.project_id === id ? { ...t, project_id: null } : t)));
    if (this.projectFilter().has(id)) this.toggleProjectInFilter(id);
    try {
      await firstValueFrom(this.api.deleteProject(id));
      this.refreshTasks();
    } catch (e) {
      this._projects.set(prevProjects);
      this._tasks.set(prevTasks);
      throw e;
    }
  }

  /** Replace the sidebar project filter. Persists across reloads like the sidebar state. */
  setProjectFilter(keys: Iterable<ProjectKey> | undefined): void {
    const next = new Set<ProjectKey>(keys ?? []);
    this._projectFilter.set(next);
    try {
      if (!next.size) localStorage.removeItem(FILTER_KEY);
      else localStorage.setItem(FILTER_KEY, JSON.stringify([...next].map(k => k ?? 'inbox')));
    } catch { /* ignore */ }
  }

  /**
   * Plain click: select just this row. Clicking the row that is already the
   * whole selection clears it.
   */
  toggleProjectFilter(key: ProjectKey): void {
    const f = this.projectFilter();
    this.setProjectFilter(f.size === 1 && f.has(key) ? undefined : [key]);
  }

  /** Ctrl/⌘-click: add this row to the selection, or drop it if it's there. */
  toggleProjectInFilter(key: ProjectKey): void {
    const next = new Set(this.projectFilter());
    if (next.has(key)) next.delete(key);
    else next.add(key);
    this.setProjectFilter(next);
  }

  setTagFilter(tag: string | undefined): void {
    this._tagFilter.set(tag);
    try {
      if (tag === undefined) localStorage.removeItem(TAG_FILTER_KEY);
      else localStorage.setItem(TAG_FILTER_KEY, tag);
    } catch { /* ignore */ }
  }

  toggleTagFilter(tag: string): void {
    this.setTagFilter(this.tagFilter() === tag ? undefined : tag);
  }

  setAssigneeFilter(who: Assignee | undefined): void {
    this._assigneeFilter.set(who);
    try {
      if (who === undefined) localStorage.removeItem(ASSIGNEE_FILTER_KEY);
      else localStorage.setItem(ASSIGNEE_FILTER_KEY, who);
    } catch { /* ignore */ }
  }

  toggleAssigneeFilter(who: Assignee): void {
    this.setAssigneeFilter(this.assigneeFilter() === who ? undefined : who);
  }

  clearFilters(): void {
    this.setProjectFilter(undefined);
    this.setTagFilter(undefined);
    this.setAssigneeFilter(undefined);
  }

  /**
   * Fill in a draft from the active filters: the project when the draft
   * doesn't name one, and the tag if it isn't already there. Creating a task
   * while filtered to "2026 Taxes" + #finance and having it vanish into
   * Inbox, untagged, would be the wrong surprise.
   */
  withFilterDefaults(draft: Partial<Task>): Partial<Task> {
    let out = draft;
    const sole = this.soleFilterProject();
    if (sole && out.project_id == null) {
      out = { ...out, project_id: sole.id };
    }
    const tag = this.tagFilter();
    if (tag !== undefined && !(out.tags ?? []).includes(tag)) {
      out = { ...out, tags: [...(out.tags ?? []), tag] };
    }
    return out;
  }

  /**
   * One-off filtered query; does NOT mutate the store's `tasks` signal.
   * Use this from views that want their own filtered slice (search) without
   * disturbing the board/list's read of the full task set.
   */
  query(filter: TaskFilter): Observable<Task[]> {
    return this.api.listTasks(filter);
  }

  async create(input: Partial<Task>): Promise<Task> {
    const created = await firstValueFrom(this.api.createTask(input));
    this._tasks.update(ts => [...ts, created]);
    return created;
  }

  async patch(id: number, patch: Partial<Task>): Promise<Task> {
    const prev = this._tasks();
    this._tasks.set(prev.map(t => (t.id === id ? { ...t, ...patch } as Task : t)));
    try {
      const updated = await firstValueFrom(this.api.patchTask(id, patch));
      this._tasks.update(ts => ts.map(t => (t.id === id ? updated : t)));
      return updated;
    } catch (e) {
      this._tasks.set(prev);
      throw e;
    }
  }

  async remove(id: number): Promise<void> {
    const prev = this._tasks();
    this._tasks.set(prev.filter(t => t.id !== id));
    try {
      await firstValueFrom(this.api.deleteTask(id));
    } catch (e) {
      this._tasks.set(prev);
      throw e;
    }
  }

  /**
   * Move a task into `targetStatus` at position `targetIndex` within that
   * column's already-sorted list. Uses fractional midpoint so reorders only
   * touch one row.
   */
  async move(taskId: number, targetStatus: Status, targetIndex: number): Promise<void> {
    const task = this._tasks().find(t => t.id === taskId);
    if (!task) return;

    const column = this.tasksByColumn().get(targetStatus) ?? [];
    const neighbors = column.filter(t => t.id !== taskId);
    const left = neighbors[targetIndex - 1];
    const right = neighbors[targetIndex];

    let sortOrder: number;
    if (!left && !right) sortOrder = 0;
    else if (!left) sortOrder = right.sort_order - 1;
    else if (!right) sortOrder = left.sort_order + 1;
    else sortOrder = (left.sort_order + right.sort_order) / 2;

    const patch: Partial<Task> = { sort_order: sortOrder };
    if (task.status !== targetStatus) patch.status = targetStatus;
    await this.patch(taskId, patch);
  }
}

function loadFilter(): ReadonlySet<ProjectKey> {
  const out = new Set<ProjectKey>();
  try {
    const v = localStorage.getItem(FILTER_KEY);
    if (v === null) return out;
    // Current shape is a JSON array like [4, "inbox"]; the first release
    // stored a bare "4" or "inbox", so accept that too.
    let raw: unknown;
    try { raw = JSON.parse(v); } catch { raw = v; }
    for (const item of Array.isArray(raw) ? raw : [raw]) {
      if (item === 'inbox') out.add(null);
      else if (Number.isInteger(Number(item))) out.add(Number(item));
    }
  } catch { /* ignore */ }
  return out;
}

function loadTagFilter(): string | undefined {
  try {
    return localStorage.getItem(TAG_FILTER_KEY) ?? undefined;
  } catch {
    return undefined;
  }
}

function loadAssigneeFilter(): Assignee | undefined {
  try {
    const v = localStorage.getItem(ASSIGNEE_FILTER_KEY);
    return v === 'me' || v === 'claude' ? v : undefined;
  } catch {
    return undefined;
  }
}
