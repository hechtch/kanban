import { Component, computed, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';

import { COLUMN_STATUSES, Project, Status, STATUS_LABEL, Task } from '../models';
import { TaskStore } from '../task-store';
import { FilterBar } from '../shared/filter-bar';
import { confirmDelete } from '../shared/confirm-delete';

interface Group {
  project: Project | null;
  tasks: Task[];
}

interface Editing {
  taskId: number;
  title: string;
  status: Status;
  priority: 0 | 1 | 2 | 3;
  due_text: string;
  project_id: number | null;
  tagsText: string;
}

@Component({
  selector: 'app-list',
  standalone: true,
  imports: [FormsModule, FilterBar],
  templateUrl: './list.html',
  styleUrl: './list.css',
})
export class List {
  private store = inject(TaskStore);

  readonly statuses = [...COLUMN_STATUSES, 'backlog' as Status];
  readonly projects = this.store.projects;

  statusLabel(s: Status): string {
    return STATUS_LABEL[s];
  }
  readonly editing = signal<Editing | null>(null);

  groups = computed<Group[]>(() => {
    const filter = this.store.projectFilter();
    const tagFilter = this.store.tagFilter();
    const byProj = new Map<number | null, Task[]>();
    for (const t of this.store.visibleTasks()) {
      const key = t.project_id;
      if (!byProj.has(key)) byProj.set(key, []);
      byProj.get(key)!.push(t);
    }
    const out: Group[] = [];
    for (const p of this.store.projects()) {
      // Unfiltered: every project gets a section, even an empty one.
      // Project filter: only the selected project's section.
      // Tag filter: only projects that actually have a matching task —
      // a tag spans projects, so most sections would otherwise be empty.
      if (filter.size && !filter.has(p.id)) continue;
      if (tagFilter !== undefined && !byProj.has(p.id)) continue;
      out.push({ project: p, tasks: (byProj.get(p.id) ?? []).slice().sort(sortKey) });
    }
    if (filter.has(null) || (!filter.size && byProj.has(null))) {
      out.push({ project: null, tasks: (byProj.get(null) ?? []).slice().sort(sortKey) });
    }
    return out;
  });

  toggleEdit(task: Task): void {
    if (this.editing()?.taskId === task.id) {
      this.editing.set(null);
      return;
    }
    this.editing.set({
      taskId: task.id,
      title: task.title,
      status: task.status,
      priority: task.priority,
      due_text: task.due_text,
      project_id: task.project_id,
      tagsText: task.tags.join(', '),
    });
  }

  cancelEdit(): void { this.editing.set(null); }

  async saveEdit(): Promise<void> {
    const e = this.editing();
    if (!e || !e.title.trim()) return;
    const tags = e.tagsText
      .split(',')
      .map(s => s.trim())
      .filter(Boolean);
    await this.store.patch(e.taskId, {
      title: e.title,
      status: e.status,
      priority: e.priority,
      due_text: e.due_text,
      project_id: e.project_id,
      tags,
    });
    this.editing.set(null);
  }

  async deleteTask(task: Task): Promise<void> {
    if (!confirmDelete(task)) return;
    await this.store.remove(task.id);
    if (this.editing()?.taskId === task.id) this.editing.set(null);
  }
}

function sortKey(a: Task, b: Task): number {
  // Group by status order, then sort_order, then id.
  const order: Record<Status, number> = {
    todo: 0, doing: 1, blocked: 2, awaiting_merge: 3, backlog: 4, done: 5,
  };
  return order[a.status] - order[b.status] || a.sort_order - b.sort_order || a.id - b.id;
}
