import { Routes } from '@angular/router';

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
    path: 'task/:id',
    loadComponent: () => import('./task-view/task-view').then(m => m.TaskView),
  },
];
