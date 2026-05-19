import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, firstValueFrom } from 'rxjs';

import { ApiService } from './api.service';
import { COLUMN_STATUSES, Project, Status, Task, TaskFilter } from './models';

@Injectable({ providedIn: 'root' })
export class TaskStore {
  private api = inject(ApiService);

  private readonly _tasks = signal<Task[]>([]);
  private readonly _projects = signal<Project[]>([]);

  readonly tasks = this._tasks.asReadonly();
  readonly projects = this._projects.asReadonly();

  readonly tasksByColumn = computed(() => {
    const out = new Map<Status, Task[]>();
    for (const s of COLUMN_STATUSES) out.set(s, []);
    for (const t of this._tasks()) {
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
    this.api.listTasks().subscribe(t => this._tasks.set(t));
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

