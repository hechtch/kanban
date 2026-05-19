import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  ViewChild,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { Subject, debounceTime, distinctUntilChanged, switchMap } from 'rxjs';

import { COLUMN_STATUSES, Project, Status, STATUS_LABEL, Task } from '../models';
import { TaskStore } from '../task-store';

@Component({
  selector: 'app-search',
  standalone: true,
  imports: [FormsModule, RouterLink],
  templateUrl: './search.html',
  styleUrl: './search.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Search {
  protected store = inject(TaskStore);
  private route = inject(ActivatedRoute);
  private router = inject(Router);

  // URL is the source of truth for filter state. Reading the params yields
  // signals; writing back through router.navigate updates them.
  private readonly params = toSignal(this.route.queryParamMap, {
    requireSync: true,
  });

  readonly query = computed(() => this.params()?.get('q') ?? '');
  readonly project = computed(() => this.params()?.get('project') ?? '');
  readonly status = computed(() => this.params()?.get('status') ?? '');

  // Drafted input value while typing (committed to URL after debounce).
  readonly draftQuery = signal('');

  readonly statuses: Status[] = [...COLUMN_STATUSES, 'backlog'];

  // Fetch results whenever any URL param changes. `undefined` means loading;
  // `[]` means we got a real empty response.
  private readonly fetch$ = new Subject<{ q: string; project: string; status: string }>();
  readonly results = toSignal<Task[]>(
    this.fetch$.pipe(
      debounceTime(0),
      distinctUntilChanged((a, b) =>
        a.q === b.q && a.project === b.project && a.status === b.status,
      ),
      switchMap(({ q, project, status }) => {
        const filter: { status?: Status; project_id?: number | null; q?: string } = {};
        if (q) filter.q = q;
        if (status) filter.status = status as Status;
        if (project) {
          const proj = this.store.projects().find(p => p.slug === project);
          if (proj) filter.project_id = proj.id;
        }
        return this.store.query(filter);
      }),
    ),
  );

  @ViewChild('input', { static: true }) private input!: ElementRef<HTMLInputElement>;

  constructor() {
    // Re-fetch whenever any URL param changes.
    effect(() => {
      this.fetch$.next({
        q: this.query(),
        project: this.project(),
        status: this.status(),
      });
    });

    // Keep the input in sync when the URL changes (e.g. back/forward).
    effect(() => {
      this.draftQuery.set(this.query());
    });
  }

  ngAfterViewInit(): void {
    // Autofocus on route entry.
    queueMicrotask(() => this.input.nativeElement.focus());
  }

  // Debounced commit of the draft text to the URL. Called on (ngModelChange).
  private debounceTimer: ReturnType<typeof setTimeout> | null = null;
  onQueryInput(value: string): void {
    this.draftQuery.set(value);
    if (this.debounceTimer !== null) clearTimeout(this.debounceTimer);
    this.debounceTimer = setTimeout(() => this.setParam('q', value), 200);
  }

  setProject(slug: string): void { this.setParam('project', slug); }
  setStatus(s: string): void { this.setParam('status', s); }

  clearAll(): void {
    this.router.navigate([], {
      relativeTo: this.route,
      queryParams: {},
      replaceUrl: false,
    });
  }

  private setParam(key: string, value: string): void {
    this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { [key]: value || null },
      queryParamsHandling: 'merge',
    });
  }

  projectFor(t: Task): Project | null {
    if (t.project_id == null) return null;
    return this.store.projects().find(p => p.id === t.project_id) ?? null;
  }

  statusLabel(s: string): string {
    return STATUS_LABEL[s as Status] ?? s;
  }

  prioLabel(p: number): string {
    return ['none', '! low', '!! med', '!!! high'][p] ?? '';
  }

  // For the "current filter description" in the empty state.
  filterSummary(): string {
    const parts: string[] = [];
    if (this.query()) parts.push(`matching "${this.query()}"`);
    if (this.project()) parts.push(`in ${this.project()}`);
    if (this.status()) parts.push(`with status ${this.statusLabel(this.status())}`);
    return parts.length ? parts.join(' ') : 'with no filter';
  }
}
