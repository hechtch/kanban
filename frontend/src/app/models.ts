export type Status =
  | 'todo'
  | 'doing'
  | 'blocked'
  | 'awaiting_merge'
  | 'done'
  | 'backlog';

export const COLUMN_STATUSES: Status[] = [
  'todo', 'doing', 'blocked', 'awaiting_merge', 'done',
];

/** Display label for a status — used by board column headers, modal selects,
 *  the menu, and the inline-list status select. */
export const STATUS_LABEL: Record<Status, string> = {
  todo: 'todo',
  doing: 'doing',
  blocked: 'blocked',
  awaiting_merge: 'awaiting merge',
  done: 'done',
  backlog: 'backlog',
};

export interface Project {
  id: number;
  slug: string;
  name: string;
  color: string;
  sort_order: number;
}

export interface Task {
  id: number;
  title: string;
  body: string;
  status: Status;
  priority: 0 | 1 | 2 | 3;
  due_text: string;
  project_id: number | null;
  sort_order: number;
  plan_slug?: string | null;
  git_branch?: string | null;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
  tags: string[];
}

export interface TaskFilter {
  status?: Status;
  project_id?: number | null;
  q?: string;
}

export interface ParsedDraft {
  title: string;
  priority: 0 | 1 | 2 | 3;
  due_text: string;
  tags: string[];
  project_name: string;
  project_id: number | null;
}
