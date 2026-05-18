import {
  Component,
  computed,
  ElementRef,
  inject,
  signal,
  ViewChildren,
  QueryList,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import {
  CdkDrag,
  CdkDragDrop,
  CdkDropList,
  CdkDropListGroup,
} from '@angular/cdk/drag-drop';

import { COLUMN_STATUSES, Project, Status, Task } from '../models';
import { TaskStore } from '../task-store';
import { Card } from './card';
import { TaskModal } from '../task-modal/task-modal';

interface MenuState {
  task: Task;
  x: number;
  y: number;
}

@Component({
  selector: 'app-board',
  standalone: true,
  imports: [
    CdkDropListGroup,
    CdkDropList,
    CdkDrag,
    FormsModule,
    Card,
    TaskModal,
  ],
  templateUrl: './board.html',
  styleUrl: './board.css',
})
export class Board {
  private store = inject(TaskStore);
  private router = inject(Router);

  readonly columns: Status[] = COLUMN_STATUSES;
  readonly projectsSig = this.store.projects;

  readonly menu = signal<MenuState | null>(null);
  readonly creating = signal<{ status: Status } | null>(null);
  readonly quickAdd = signal<Record<Status, string>>({
    todo: '', doing: '', waiting: '', done: '', backlog: '',
  });

  private readonly projectsById = computed(() => {
    const m = new Map<number, Project>();
    for (const p of this.store.projects()) m.set(p.id, p);
    return m;
  });

  @ViewChildren('cardEl', { read: ElementRef })
  private cardEls!: QueryList<ElementRef<HTMLElement>>;

  tasksFor(col: Status): Task[] {
    return this.store.tasksByColumn().get(col) ?? [];
  }

  projectFor(t: Task): Project | null {
    return t.project_id == null ? null : this.projectsById().get(t.project_id) ?? null;
  }

  // ─── drag-drop ────────────────────────────────────────────────────
  async onDrop(event: CdkDragDrop<Task[]>, targetCol: Status): Promise<void> {
    const task = event.item.data as Task;
    if (event.previousContainer === event.container && event.previousIndex === event.currentIndex) {
      return;
    }
    await this.store.move(task.id, targetCol, event.currentIndex);
  }

  // ─── quick menu ───────────────────────────────────────────────────
  openMenu(event: MouseEvent, task: Task): void {
    event.preventDefault();
    event.stopPropagation();
    this.menu.set({ task, x: event.clientX, y: event.clientY });
  }

  closeMenu(): void { this.menu.set(null); }

  async cycleStatus(task: Task): Promise<void> {
    const idx = this.columns.indexOf(task.status);
    const next = this.columns[(idx + 1) % this.columns.length];
    await this.setStatus(task, next);
  }

  async setStatus(task: Task, status: Status): Promise<void> {
    if (task.status === status) return;
    await this.store.move(task.id, status, this.tasksFor(status).length);
  }

  async setPriority(task: Task, priority: number): Promise<void> {
    await this.store.patch(task.id, { priority: priority as Task['priority'] });
  }

  async remove(task: Task): Promise<void> { await this.store.remove(task.id); }

  // ─── inline quick-add ─────────────────────────────────────────────
  async submitQuickAdd(col: Status): Promise<void> {
    const draft = this.quickAdd();
    const title = (draft[col] ?? '').trim();
    if (!title) return;
    this.quickAdd.set({ ...draft, [col]: '' });
    await this.store.create({ title, status: col });
  }

  quickAddInput(col: Status, value: string): void {
    this.quickAdd.set({ ...this.quickAdd(), [col]: value });
  }

  // ─── modals ───────────────────────────────────────────────────────
  openNew(col: Status = 'todo'): void {
    this.creating.set({ status: col });
  }

  openEdit(task: Task): void {
    this.router.navigate(['/task', task.id]);
  }

  async onModalSave(patch: Partial<Task>): Promise<void> {
    if (this.creating()) {
      await this.store.create(patch);
      this.creating.set(null);
    }
  }

  closeModals(): void {
    this.creating.set(null);
  }

  onKeydown(event: KeyboardEvent): void {
    const target = event.target as HTMLElement;
    const taskIdAttr = target.getAttribute?.('data-task-id');
    if (!taskIdAttr) return;
    const taskId = Number(taskIdAttr);
    const task = this.store.tasks().find(t => t.id === taskId);
    if (!task) return;

    if (event.key === ' ' || event.key === 'Spacebar') {
      event.preventDefault();
      this.cycleStatus(task);
      return;
    }
    if (event.key >= '1' && event.key <= '4') {
      event.preventDefault();
      this.setPriority(task, Number(event.key) - 1);
      return;
    }
    if (event.key === 'e' || event.key === 'Enter') {
      event.preventDefault();
      this.openEdit(task);
      return;
    }
    if (event.key === 'Delete' || event.key === 'Backspace') {
      event.preventDefault();
      this.remove(task);
      return;
    }
    if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)) {
      event.preventDefault();
      this.moveFocus(task, event.key);
    }
  }

  private moveFocus(task: Task, key: string): void {
    const col = task.status;
    const colIdx = this.columns.indexOf(col);
    const list = this.tasksFor(col);
    const idx = list.findIndex(t => t.id === task.id);

    let targetId: number | undefined;
    if (key === 'ArrowUp' && idx > 0) targetId = list[idx - 1].id;
    else if (key === 'ArrowDown' && idx < list.length - 1) targetId = list[idx + 1].id;
    else if (key === 'ArrowLeft' || key === 'ArrowRight') {
      const dir = key === 'ArrowLeft' ? -1 : 1;
      const nextCol = this.columns[colIdx + dir];
      if (!nextCol) return;
      const nextList = this.tasksFor(nextCol);
      if (!nextList.length) return;
      targetId = nextList[Math.min(idx, nextList.length - 1)].id;
    }

    if (targetId == null) return;
    const el = this.cardEls.find(
      ref => ref.nativeElement.getAttribute('data-task-id') === String(targetId)
    );
    el?.nativeElement.focus();
  }
}
