// Every delete path — ticket modal, board menu, board Delete/Backspace key,
// list editor — goes through this so a task is never dropped without a
// prompt. Deletes are optimistic and have no undo, and the board's
// keyboard path in particular is one stray Backspace away.

import { Task } from '../models';

export function confirmDelete(task: Pick<Task, 'title'>): boolean {
  return confirm(`Delete "${task.title}"?`);
}
