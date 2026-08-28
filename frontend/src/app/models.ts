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
  /** Finished: folded away in the sidebar, tasks off the board unless the
   *  project is explicitly selected. Nothing is deleted. */
  archived: boolean;
  /** Tags every task in the project carries. The server merges them into
   *  each task's `tags` on read; they aren't stored on the task itself. */
  tags: string[];
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
  /** Suggested Claude model for an agent picking this up (e.g. `fable`, `sonnet`). */
  model?: string | null;
  /** Suggested reasoning effort to pair with `model` (`low` … `max`). */
  effort?: string | null;
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

/** Model shorthands offered in the ticket's Model select. Free-form on the
 *  server — these are just the house set, most capable first. */
export const MODEL_OPTIONS = ['fable', 'opus', 'sonnet', 'haiku'] as const;

/** Effort tiers, matching Claude Code's reasoning-effort levels. Enforced
 *  server-side. */
export const EFFORT_OPTIONS = ['low', 'medium', 'high', 'xhigh', 'max'] as const;

export type Assignee = 'me' | 'claude';

/**
 * Who owns a task. Derived, never stored: a task carrying a `plan_slug`
 * was claimed through the agent API, and that IS what "Claude's" means
 * here. The card's actor chip renders from this same call, so the sidebar
 * filter and the chip can't drift apart.
 */
export function assigneeOf(task: Pick<Task, 'plan_slug'>): Assignee {
  return task.plan_slug != null ? 'claude' : 'me';
}

/** Sidebar/pill wording. Matches the card chip, which addresses the reader. */
export const ASSIGNEE_LABEL: Record<Assignee, string> = {
  me: 'you',
  claude: 'Claude',
};

export const ASSIGNEES: Assignee[] = ['me', 'claude'];
