import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

import { TaskStore } from './task-store';
import { Project, Task } from './models';

const projects: Project[] = [
  { id: 1, slug: 'taxes', name: '2026 Taxes', color: '#a00', sort_order: 0, archived: false, tags: [] },
  { id: 2, slug: 'house', name: 'House', color: '#0a0', sort_order: 1, archived: false, tags: [] },
];

function task(
  id: number, project_id: number | null, status: Task['status'] = 'todo', tags: string[] = [],
  plan_slug: string | null = null,
): Task {
  return {
    id, title: `t${id}`, body: '', status, priority: 0, due_text: '', project_id,
    sort_order: id, created_at: '', updated_at: '', completed_at: null, tags, plan_slug,
  };
}

// #finance spans Taxes (10) and House (20); 11 is a Taxes dev-ish task
// without it, 30 is an untagged Inbox task.
const tasks: Task[] = [
  task(10, 1, 'todo', ['finance']),
  task(11, 1, 'doing', ['dev']),
  task(20, 2, 'todo', ['finance', 'dev']),
  task(30, null),
];

describe('TaskStore project filter', () => {
  let store: TaskStore;
  let http: HttpTestingController;

  beforeEach(() => {
    localStorage.removeItem('kanban-project-filter');
    localStorage.removeItem('kanban-tag-filter');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    http = TestBed.inject(HttpTestingController);
    store = TestBed.inject(TaskStore);
    http.expectOne(req => req.url.endsWith('/api/projects')).flush(projects);
    http.expectOne(req => req.url.endsWith('/api/tasks')).flush(tasks);
  });

  afterEach(() => {
    http.verify();
    localStorage.removeItem('kanban-project-filter');
    localStorage.removeItem('kanban-tag-filter');
  });

  it('shows everything by default', () => {
    expect(store.projectFilter().size).toBe(0);
    expect(store.visibleTasks().map(t => t.id)).toEqual([10, 11, 20, 30]);
  });

  it('narrows visible tasks and columns to the selected project', () => {
    store.setProjectFilter([1]);
    expect(store.visibleTasks().map(t => t.id)).toEqual([10, 11]);
    expect(store.tasksByColumn().get('todo')!.map(t => t.id)).toEqual([10]);
    expect(store.tasksByColumn().get('doing')!.map(t => t.id)).toEqual([11]);
    expect(store.soleFilterProject()?.name).toBe('2026 Taxes');
    expect(store.filterProjects().map(p => p.name)).toEqual(['2026 Taxes']);
    // The full set is untouched.
    expect(store.tasks().length).toBe(4);
  });

  it('null selects Inbox (tasks with no project)', () => {
    store.setProjectFilter([null]);
    expect(store.visibleTasks().map(t => t.id)).toEqual([30]);
    expect(store.inboxSelected()).toBe(true);
    expect(store.soleFilterProject()).toBeNull();
  });

  it('plain click selects one row; clicking the sole selection clears', () => {
    store.toggleProjectFilter(2);
    expect([...store.projectFilter()]).toEqual([2]);
    store.toggleProjectFilter(2);
    expect(store.projectFilter().size).toBe(0);
    store.toggleProjectFilter(2);
    store.toggleProjectFilter(1);
    expect([...store.projectFilter()]).toEqual([1]);
  });

  it('ctrl-click adds and removes rows; several projects OR together', () => {
    store.toggleProjectFilter(1);
    store.toggleProjectInFilter(2);
    expect(store.visibleTasks().map(t => t.id)).toEqual([10, 11, 20]);
    expect(store.filterProjects().map(p => p.id)).toEqual([1, 2]);
    // Several projects selected → no single project to default new tasks into.
    expect(store.soleFilterProject()).toBeNull();
    expect(store.withFilterDefaults({ title: 'x' }).project_id).toBeUndefined();
    store.toggleProjectInFilter(null);
    expect(store.visibleTasks().map(t => t.id)).toEqual([10, 11, 20, 30]);
    store.toggleProjectInFilter(1);
    expect(store.visibleTasks().map(t => t.id)).toEqual([20, 30]);
    // Plain click on a row inside a multi-selection narrows to just it.
    store.toggleProjectFilter(2);
    expect([...store.projectFilter()]).toEqual([2]);
  });

  it('persists the selection to localStorage', () => {
    store.setProjectFilter([2]);
    expect(localStorage.getItem('kanban-project-filter')).toBe('[2]');
    store.setProjectFilter([2, null]);
    expect(localStorage.getItem('kanban-project-filter')).toBe('[2,"inbox"]');
    store.setProjectFilter(undefined);
    expect(localStorage.getItem('kanban-project-filter')).toBeNull();
  });

  it('drops persisted projects that no longer exist', () => {
    store.setProjectFilter([999]);
    expect(store.projectFilter().size).toBe(0);
    expect(store.visibleTasks().length).toBe(4);
    store.setProjectFilter([999, 1]);
    expect([...store.projectFilter()]).toEqual([1]);
  });

  it('defaults a draft into the filtered project unless it names one', () => {
    store.setProjectFilter([1]);
    expect(store.withFilterDefaults({ title: 'x' }).project_id).toBe(1);
    expect(store.withFilterDefaults({ title: 'x', project_id: null }).project_id).toBe(1);
    expect(store.withFilterDefaults({ title: 'x', project_id: 2 }).project_id).toBe(2);
    store.setProjectFilter([null]);
    expect(store.withFilterDefaults({ title: 'x' }).project_id).toBeUndefined();
    store.setProjectFilter(undefined);
    expect(store.withFilterDefaults({ title: 'x' }).project_id).toBeUndefined();
  });

  // ── tags ────────────────────────────────────────────────────────────

  it('counts tags across all tasks, most-used first', () => {
    expect(store.tagCounts()).toEqual([
      { name: 'dev', count: 2 },
      { name: 'finance', count: 2 },
    ]);
  });

  it('a tag filter spans projects', () => {
    store.setTagFilter('finance');
    expect(store.visibleTasks().map(t => t.id)).toEqual([10, 20]);
    expect(store.hasFilter()).toBe(true);
  });

  it('project and tag filters AND together', () => {
    store.setProjectFilter([1]);
    store.setTagFilter('finance');
    expect(store.visibleTasks().map(t => t.id)).toEqual([10]);
    store.setTagFilter('dev');
    expect(store.visibleTasks().map(t => t.id)).toEqual([11]);
    store.clearFilters();
    expect(store.hasFilter()).toBe(false);
    expect(store.visibleTasks().length).toBe(4);
  });

  it('toggling the active tag clears it and the choice persists', () => {
    store.toggleTagFilter('dev');
    expect(store.tagFilter()).toBe('dev');
    expect(localStorage.getItem('kanban-tag-filter')).toBe('dev');
    store.toggleTagFilter('dev');
    expect(store.tagFilter()).toBeUndefined();
    expect(localStorage.getItem('kanban-tag-filter')).toBeNull();
  });

  it('a tag nothing carries any more falls back to no filter', () => {
    store.setTagFilter('gone');
    expect(store.tagFilter()).toBeUndefined();
    expect(store.visibleTasks().length).toBe(4);
  });

  it('adds the filtered tag to new drafts without duplicating it', () => {
    store.setTagFilter('finance');
    expect(store.withFilterDefaults({ title: 'x' }).tags).toEqual(['finance']);
    expect(store.withFilterDefaults({ title: 'x', tags: ['urgent'] }).tags).toEqual(['urgent', 'finance']);
    expect(store.withFilterDefaults({ title: 'x', tags: ['finance'] }).tags).toEqual(['finance']);
    store.setTagFilter(undefined);
    expect(store.withFilterDefaults({ title: 'x' }).tags).toBeUndefined();
  });
});

describe('TaskStore archived projects', () => {
  let store: TaskStore;
  let http: HttpTestingController;

  // 2025 Taxes is finished and archived; its one task carries #tax via the project.
  const withArchived: Project[] = [
    ...projects,
    { id: 3, slug: 'taxes-2025', name: '2025 Taxes', color: '#00a', sort_order: 2, archived: true, tags: ['tax'] },
  ];
  const tasksWithArchived: Task[] = [...tasks, task(40, 3, 'done', ['tax'])];

  beforeEach(() => {
    localStorage.removeItem('kanban-project-filter');
    localStorage.removeItem('kanban-tag-filter');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    http = TestBed.inject(HttpTestingController);
    store = TestBed.inject(TaskStore);
    http.expectOne(req => req.url.endsWith('/api/projects')).flush(withArchived);
    http.expectOne(req => req.url.endsWith('/api/tasks')).flush(tasksWithArchived);
  });

  afterEach(() => {
    http.verify();
    localStorage.removeItem('kanban-project-filter');
  });

  it('splits projects into active and archived', () => {
    expect(store.activeProjects().map(p => p.id)).toEqual([1, 2]);
    expect(store.archivedProjects().map(p => p.id)).toEqual([3]);
  });

  it('keeps archived tasks and their tags off the board by default', () => {
    expect(store.visibleTasks().map(t => t.id)).toEqual([10, 11, 20, 30]);
    expect(store.activeTasks().map(t => t.id)).toEqual([10, 11, 20, 30]);
    expect(store.tagCounts().map(t => t.name)).not.toContain('tax');
  });

  it('shows an archived project when it is picked explicitly', () => {
    store.setProjectFilter([3]);
    expect(store.visibleTasks().map(t => t.id)).toEqual([40]);
    expect(store.tagCounts().map(t => t.name)).toContain('tax');
  });

  it('re-reads tasks after a project tag change, since the server merges them', async () => {
    const pending = store.patchProject(1, { tags: ['finance'] });
    http.expectOne(req => req.method === 'PATCH' && req.url.endsWith('/api/projects/1'))
      .flush({ ...projects[0], tags: ['finance'] });
    await pending;
    http.expectOne(req => req.url.endsWith('/api/tasks')).flush(tasksWithArchived);
    expect(store.projects().find(p => p.id === 1)?.tags).toEqual(['finance']);
  });
});

describe('TaskStore assignee filter', () => {
  let store: TaskStore;
  let http: HttpTestingController;

  // A task with a plan_slug was claimed through the agent API — that's
  // what makes it Claude's, and it's the same signal the card chip uses.
  const mixed: Task[] = [
    task(10, 1, 'todo', []),
    task(11, 1, 'doing', [], 'agent-api'),
    task(12, null, 'todo', [], 'search'),
  ];

  beforeEach(() => {
    localStorage.removeItem('kanban-project-filter');
    localStorage.removeItem('kanban-tag-filter');
    localStorage.removeItem('kanban-assignee-filter');
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    http = TestBed.inject(HttpTestingController);
    store = TestBed.inject(TaskStore);
    http.expectOne(req => req.url.endsWith('/api/projects')).flush(projects);
    http.expectOne(req => req.url.endsWith('/api/tasks')).flush(mixed);
  });

  afterEach(() => {
    http.verify();
    localStorage.removeItem('kanban-assignee-filter');
  });

  it('counts plan-owned tasks as Claude and the rest as yours', () => {
    expect(store.assigneeCounts()).toEqual({ me: 1, claude: 2 });
  });

  it('narrows to one assignee and back', () => {
    store.setAssigneeFilter('claude');
    expect(store.visibleTasks().map(t => t.id)).toEqual([11, 12]);
    store.setAssigneeFilter('me');
    expect(store.visibleTasks().map(t => t.id)).toEqual([10]);
    store.toggleAssigneeFilter('me');
    expect(store.assigneeFilter()).toBeUndefined();
    expect(store.visibleTasks().length).toBe(3);
  });

  it('ANDs with the project filter', () => {
    store.setProjectFilter([1]);
    store.setAssigneeFilter('claude');
    expect(store.visibleTasks().map(t => t.id)).toEqual([11]);
    expect(store.hasFilter()).toBe(true);
  });

  it('persists across reloads and is cleared by clearFilters', () => {
    store.setAssigneeFilter('claude');
    expect(localStorage.getItem('kanban-assignee-filter')).toBe('claude');
    store.clearFilters();
    expect(store.assigneeFilter()).toBeUndefined();
    expect(localStorage.getItem('kanban-assignee-filter')).toBeNull();
  });
});
