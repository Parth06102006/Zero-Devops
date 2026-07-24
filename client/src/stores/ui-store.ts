import { create } from "zustand";

interface UIState {
  isMobileNavOpen: boolean;
  isSidebarCollapsed: boolean;
}

interface UIActions {
  setMobileNavOpen: (open: boolean) => void;
  toggleSidebar: () => void;
}

/**
 * Purely client-side, ephemeral UI state that doesn't belong to any single
 * feature and never touches the network. Server data belongs in TanStack
 * Query, not here.
 */
export const useUIStore = create<UIState & UIActions>((set) => ({
  isMobileNavOpen: false,
  isSidebarCollapsed: false,

  setMobileNavOpen: (open) => set({ isMobileNavOpen: open }),
  toggleSidebar: () => set((state) => ({ isSidebarCollapsed: !state.isSidebarCollapsed })),
}));
