import { vi } from 'vitest';

/**
 * File: tests/setup.ts
 * Purpose: Global Vitest setup for mocking Wails environment and internationalization.
 */

// Force locale to en-US for consistent number and date formatting
// We override the default behavior of toLocaleString to ensure consistency between local Dev and CI
const originalNumberToLocaleString = Number.prototype.toLocaleString;
Number.prototype.toLocaleString = function(locales?: string | string[], options?: Intl.NumberFormatOptions) {
  return originalNumberToLocaleString.call(this, locales || 'en-US', options);
};

// Also for Date if needed (used in StatsView)
const originalDateToLocaleString = Date.prototype.toLocaleString;
Date.prototype.toLocaleString = function(locales?: string | string[], options?: Intl.DateTimeFormatOptions) {
  return originalDateToLocaleString.call(this, locales || 'en-US', options);
};

// Mock window.wails for the real runtime to not crash
if (typeof window !== 'undefined') {
  (window as any).wails = {
    EventsOn: vi.fn(),
    EventsOnMultiple: vi.fn(),
    EventsOnce: vi.fn(),
    EventsOff: vi.fn(),
    EventsEmit: vi.fn(),
    LogDebug: vi.fn(),
    LogInfo: vi.fn(),
    LogWarning: vi.fn(),
    LogError: vi.fn(),
    WindowReload: vi.fn(),
    WindowReloadApp: vi.fn(),
    WindowSetSystemDefaultTheme: vi.fn(),
    WindowSetLightTheme: vi.fn(),
    WindowSetDarkTheme: vi.fn(),
    WindowShow: vi.fn(),
    WindowHide: vi.fn(),
    WindowCenter: vi.fn(),
    WindowMaximise: vi.fn(),
    WindowUnmaximise: vi.fn(),
    WindowMinimise: vi.fn(),
    WindowUnminimise: vi.fn(),
    WindowSetAlwaysOnTop: vi.fn(),
    WindowSetSize: vi.fn(),
    WindowSetMinSize: vi.fn(),
    WindowSetMaxSize: vi.fn(),
    WindowSetTitle: vi.fn(),
    WindowSetPosition: vi.fn(),
    WindowGetPosition: vi.fn().mockResolvedValue({ x: 0, y: 0 }),
    WindowGetSize: vi.fn().mockResolvedValue({ w: 1024, h: 768 }),
    WindowIsMaximised: vi.fn().mockResolvedValue(false),
    WindowIsMinimised: vi.fn().mockResolvedValue(false),
    WindowIsNormal: vi.fn().mockResolvedValue(true),
    WindowIsFullscreen: vi.fn().mockResolvedValue(false),
  };

  // Mock window.runtime
  (window as any).runtime = (window as any).wails;

  // Mock window.go for Wails bindings
  (window as any).go = {
    main: {
      App: new Proxy({}, {
        get: () => vi.fn().mockResolvedValue({})
      })
    }
  };
}

// Global mock for wailsjs/runtime/runtime to prevent imports from crashing
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn().mockReturnValue(() => {}),
  EventsOnMultiple: vi.fn().mockReturnValue(() => {}),
  EventsOnce: vi.fn().mockReturnValue(() => {}),
  EventsOff: vi.fn(),
  EventsEmit: vi.fn(),
  LogDebug: vi.fn(),
  LogInfo: vi.fn(),
  LogWarning: vi.fn(),
  LogError: vi.fn(),
  WindowReload: vi.fn(),
  WindowCenter: vi.fn(),
  WindowSetTitle: vi.fn(),
  WindowSetSize: vi.fn(),
  WindowGetSize: vi.fn().mockResolvedValue({ w: 1024, h: 768 }),
}));
