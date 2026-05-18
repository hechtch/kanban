import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';

import { ParsedDraft, Project, Task, TaskFilter } from './models';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private http = inject(HttpClient);

  // Derive from <base href> so the app works both standalone (base="/")
  // and behind the dashboard reverse proxy (base="/apps/kanban/").
  // Do NOT use APP_BASE_HREF — it's not provided by default and silently
  // returns null, which drops the /apps/<name>/ prefix.
  private readonly base = (
    document.querySelector('base')?.getAttribute('href') ?? '/'
  ).replace(/\/+$/, '') + '/api';

  health(): Observable<{ status: string }> {
    return this.http.get<{ status: string }>(`${this.base}/health`);
  }

  listProjects(): Observable<Project[]> {
    return this.http.get<Project[]>(`${this.base}/projects`);
  }

  createProject(p: Partial<Project>): Observable<Project> {
    return this.http.post<Project>(`${this.base}/projects`, p);
  }

  patchProject(id: number, patch: Partial<Project>): Observable<Project> {
    return this.http.patch<Project>(`${this.base}/projects/${id}`, patch);
  }

  deleteProject(id: number): Observable<void> {
    return this.http.delete<void>(`${this.base}/projects/${id}`);
  }

  listTasks(filter: TaskFilter = {}): Observable<Task[]> {
    let params = new HttpParams();
    if (filter.status) params = params.set('status', filter.status);
    if (filter.project_id !== undefined) {
      params = params.set('project_id', filter.project_id === null ? 'null' : String(filter.project_id));
    }
    if (filter.q) params = params.set('q', filter.q);
    return this.http.get<Task[]>(`${this.base}/tasks`, { params });
  }

  createTask(t: Partial<Task>): Observable<Task> {
    return this.http.post<Task>(`${this.base}/tasks`, t);
  }

  parseTask(text: string): Observable<ParsedDraft> {
    return this.http.post<ParsedDraft>(`${this.base}/tasks/parse`, { text });
  }

  patchTask(id: number, patch: Partial<Task>): Observable<Task> {
    return this.http.patch<Task>(`${this.base}/tasks/${id}`, patch);
  }

  deleteTask(id: number): Observable<void> {
    return this.http.delete<void>(`${this.base}/tasks/${id}`);
  }
}
