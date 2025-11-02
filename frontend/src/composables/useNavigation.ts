/*
  File: composables/useNavigation.ts
  Purpose: Simple navigation state management without router dependencies.
  Author: CodeTextor project
  Notes: Provides reactive navigation state for the application.
*/

import { ref } from 'vue';

// Available views in the application
export type ViewName = 'projects' | 'indexing' | 'search' | 'outline' | 'stats' | 'mcp';

// Current active view
const currentView = ref<ViewName>('indexing');

/**
 * Composable for managing navigation state.
 * Provides reactive current view and navigation method.
 * @returns Navigation state and methods
 */
export function useNavigation() {
  /**
   * Navigates to a specified view.
   * @param view - Target view name
   */
  const navigateTo = (view: ViewName) => {
    currentView.value = view;
  };

  return {
    currentView,
    navigateTo
  };
}
