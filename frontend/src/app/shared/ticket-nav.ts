// The ticket modal is addressed by a `?task=<id>` query param on whatever
// view is underneath (`/board?task=5`, `/list?task=5`), so the board stays
// visible behind it and the URL is still a deep link. `/task/<id>` — the
// URL agents write into notes — redirects here (see app.routes.ts).

import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';

export const TICKET_PARAM = 'task';

@Injectable({ providedIn: 'root' })
export class TicketNav {
  private router = inject(Router);

  /** Open the ticket over the current view, keeping the view's own params. */
  open(id: number): Promise<boolean> {
    return this.router.navigate([], {
      queryParams: { [TICKET_PARAM]: id },
      queryParamsHandling: 'merge',
    });
  }

  /** Close the ticket; the view underneath is untouched. */
  close(): Promise<boolean> {
    return this.router.navigate([], {
      queryParams: { [TICKET_PARAM]: null },
      queryParamsHandling: 'merge',
    });
  }
}
