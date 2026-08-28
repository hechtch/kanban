import { TestBed } from '@angular/core/testing';
import { Router, provideRouter } from '@angular/router';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';

import { App } from './app';
import { routes } from './app.routes';

describe('App', () => {
  let http: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [
        provideRouter(routes),
        provideHttpClient(),
        provideHttpClientTesting(),
      ],
    }).compileComponents();
    http = TestBed.inject(HttpTestingController);
  });

  function drainInitialLoads(): void {
    http.expectOne(req => req.url.endsWith('/api/projects')).flush([]);
    http.expectOne(req => req.url.endsWith('/api/tasks')).flush([]);
  }

  it('should create the app', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    expect(fixture.componentInstance).toBeTruthy();
    drainInitialLoads();
  });

  it('/task/<id> deep links open the ticket as a modal over the board', async () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    drainInitialLoads();
    const router = TestBed.inject(Router);

    await router.navigateByUrl('/task/42');
    expect(router.url).toBe('/board?task=42');
    expect(fixture.componentInstance.ticketId()).toBe(42);
  });

  it('picking a project from a ticket closes it, leaving the filtered board', async () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    drainInitialLoads();
    const router = TestBed.inject(Router);

    await router.navigateByUrl('/list?task=42');
    expect(fixture.componentInstance.ticketId()).toBe(42);

    fixture.componentInstance.selectProject(7);
    await fixture.whenStable();
    expect(router.url).toBe('/list');
    expect(fixture.componentInstance.ticketId()).toBeNull();
    expect([...fixture.componentInstance.filter()]).toEqual([7]);

    // No ticket open: selecting just changes the filter, view stays put.
    fixture.componentInstance.selectProject(3);
    await fixture.whenStable();
    expect(router.url).toBe('/list');
    fixture.componentInstance.selectAll();
  });

  it('picking a project from Search jumps to the board', async () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    drainInitialLoads();
    const router = TestBed.inject(Router);

    await router.navigateByUrl('/search?q=merger');
    fixture.componentInstance.selectProject(7);
    await fixture.whenStable();
    expect(router.url).toBe('/board');
    expect([...fixture.componentInstance.filter()]).toEqual([7]);
    fixture.componentInstance.selectAll();
  });

  it('should render the Kanban heading', () => {
    const fixture = TestBed.createComponent(App);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('h1')?.textContent).toContain('Kanban');
    drainInitialLoads();
  });
});
