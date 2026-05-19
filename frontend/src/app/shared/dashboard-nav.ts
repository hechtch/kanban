// Shared dashboard return menu — adapted from ~/.claude/design/shared/
// for the kanban's existing CSS-variable set (--paper, --ink, --hairline,
// etc.) and Angular control-flow syntax. Renders a "⌂ dashboard ▾" pill
// that opens a dropdown listing all dashboards and apps, with the current
// app marked. Only visible when running under /apps/<slug>/.
//
// Outside-click closure is the host's responsibility — the host listens on
// document:click and clears its `navMenuOpen` signal.

import { Component, EventEmitter, Input, OnInit, Output, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';

interface DashApp { name: string; title: string; description?: string; }
interface DashGroup { key: string; title: string; apps: DashApp[]; }
interface AppsManifest { groups: Record<string, { title: string; apps: DashApp[] }>; }

const BASE_HREF = document.querySelector('base')?.getAttribute('href') ?? '/';
const CURRENT_APP_SLUG = (() => {
  const m = BASE_HREF.match(/^\/apps\/([^/]+)\//);
  return m ? m[1] : '';
})();

@Component({
  selector: 'app-dashboard-nav',
  standalone: true,
  template: `
    @if (showDashboardLink) {
      <div class="dash-nav">
        <button class="dash-home"
                (click)="onToggle($event)"
                [attr.aria-expanded]="open"
                aria-haspopup="true">⌂ dashboard ▾</button>
        @if (open) {
          <div class="dash-menu" (click)="$event.stopPropagation()">
            <a class="dash-menu__item dash-menu__item--home" href="/">⌂ All Dashboards</a>
            @for (group of dashboardGroups; track group.key) {
              <div class="dash-menu__sep"></div>
              <a class="dash-menu__group" [href]="'/' + group.key + '/'">{{ group.title }}</a>
              @for (app of group.apps; track app.name) {
                <a class="dash-menu__item"
                   [class.dash-menu__item--current]="app.name === currentApp"
                   [href]="'/apps/' + app.name + '/'">
                  {{ app.title }}
                  @if (app.name === currentApp) {
                    <span class="dash-menu__current-tag">current</span>
                  }
                </a>
              }
            }
          </div>
        }
      </div>
    }
  `,
  styles: [`
    .dash-nav { position: relative; }
    .dash-home {
      font-family: var(--font-hand);
      font-size: 0.95rem;
      color: var(--muted);
      background: transparent;
      border: 1.25px solid var(--hairline);
      border-radius: 14px;
      padding: 3px 12px;
      letter-spacing: 0.3px;
      white-space: nowrap;
      cursor: pointer;
      transition: color 120ms, border-color 120ms;
    }
    .dash-home:hover { color: var(--ink); border-color: var(--ink); }
    .dash-menu {
      position: absolute;
      top: calc(100% + 4px);
      right: 0;
      min-width: 240px;
      background: var(--paper);
      border: 1.25px solid var(--ink);
      border-radius: 4px;
      padding: 4px 0;
      z-index: 50;
      box-shadow: 2px 3px 0 var(--hairline);
    }
    .dash-menu__item {
      display: flex;
      justify-content: space-between;
      align-items: baseline;
      padding: 6px 14px;
      font-size: 0.82rem;
      color: var(--ink);
      text-decoration: none;
    }
    .dash-menu__item:hover { background: #fff; }
    .dash-menu__item--home { font-weight: 600; }
    .dash-menu__item--current { color: var(--muted); pointer-events: none; }
    .dash-menu__current-tag {
      font-size: 0.7rem;
      color: var(--muted);
      font-style: italic;
      margin-left: 6px;
    }
    .dash-menu__group {
      display: block;
      padding: 6px 14px 2px;
      font-size: 0.68rem;
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: var(--muted);
      text-decoration: none;
    }
    .dash-menu__group:hover { color: var(--ink); }
    .dash-menu__sep {
      height: 1px;
      background: var(--hairline);
      margin: 4px 0;
    }
  `],
})
export class DashboardNav implements OnInit {
  @Input() open = false;
  @Output() toggle = new EventEmitter<MouseEvent>();

  private http = inject(HttpClient);

  readonly showDashboardLink = BASE_HREF.startsWith('/apps/');
  readonly currentApp = CURRENT_APP_SLUG;
  dashboardGroups: DashGroup[] = [];

  ngOnInit(): void {
    if (this.showDashboardLink) {
      this.http.get<AppsManifest>('/apps.json').subscribe({
        next: m => {
          this.dashboardGroups = Object.entries(m.groups ?? {}).map(([key, g]) => ({
            key, title: g.title, apps: g.apps ?? [],
          }));
        },
        error: () => { this.dashboardGroups = []; },
      });
    }
  }

  onToggle(event: MouseEvent): void {
    event.stopPropagation();
    this.toggle.emit(event);
  }
}
