import { Component, HostListener, computed, inject, signal } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

import { Task } from './models';
import { TaskStore } from './task-store';
import { Capture } from './task-modal/capture';
import { DashboardNav } from './shared/dashboard-nav';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, Capture, DashboardNav],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App {
  private store = inject(TaskStore);
  private router = inject(Router);

  // <base href> like "/apps/kanban/" means we're hosted inside the dashboard
  // reverse proxy; show a back-link to "/".
  private readonly baseHref = document.querySelector('base')?.getAttribute('href') ?? '/';
  readonly inDashboard = this.baseHref.startsWith('/apps/');

  readonly projects = this.store.projects;
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

  openCapture(): void { this.captureOpen.set(true); }
  closeCapture(): void { this.captureOpen.set(false); }

  async onCaptureCreate(draft: Partial<Task>): Promise<void> {
    this.captureOpen.set(false);
    await this.store.create(draft);
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
