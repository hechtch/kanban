export type Status = 'todo' | 'doing' | 'waiting' | 'done' | 'backlog';

export const COLUMN_STATUSES: Status[] = ['todo', 'doing', 'waiting', 'done'];

export interface Project {
  id: number;
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
