import { inject } from '@angular/core';
import { Router, Routes } from '@angular/router';

import { TICKET_PARAM } from './shared/ticket-nav';

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'board' },
  {
    path: 'board',
    loadComponent: () => import('./board/board').then(m => m.Board),
  },
  {
    path: 'list',
    loadComponent: () => import('./list/list').then(m => m.List),
  },
  {
    path: 'search',
    loadComponent: () => import('./search/search').then(m => m.Search),
  },
  {
    // Legacy deep link (agents write `/task/<id>` into notes). The ticket is
    // now a modal over the board, addressed by `?task=<id>`.
    path: 'task/:id',
    redirectTo: ({ params }) =>
      inject(Router).createUrlTree(['/board'], { queryParams: { [TICKET_PARAM]: params['id'] } }),
  },
];
