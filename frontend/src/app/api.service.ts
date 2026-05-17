import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';

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

  health() {
    return this.http.get<{ status: string }>(`${this.base}/health`);
  }
}
