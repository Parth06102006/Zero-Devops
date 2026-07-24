import { create } from "zustand";

import type { ToastVariant } from "@/components/ui/toast";

export interface ToastItem {
  id: string;
  title: string;
  description?: string;
  variant?: ToastVariant;
}

interface ToastState {
  toasts: ToastItem[];
  push: (toast: Omit<ToastItem, "id">) => void;
  dismiss: (id: string) => void;
}

export const useToastStore = create<ToastState>((set) => ({
  toasts: [],
  push: (toast) =>
    set((state) => ({
      toasts: [...state.toasts, { ...toast, id: crypto.randomUUID() }],
    })),
  dismiss: (id) =>
    set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) })),
}));

/**
 * Imperative helper for firing a toast from anywhere (hooks, services,
 * event handlers) without needing to be inside a component that subscribes
 * to the store.
 */
export function toast(input: Omit<ToastItem, "id">): void {
  useToastStore.getState().push(input);
}
