import { create } from "zustand";

type Page = "main" | "settings";

interface UIState {
  currentPage: Page;
  expandedNetwork: string | null;
  networkFormOpen: boolean;
  serviceFormOpen: boolean;
  editingNetworkIndex: number | null;
  editingServiceType: "portproxy" | "socks5" | "rdp" | null;
  editingServiceIndex: number | null;

  setCurrentPage: (page: Page) => void;
  setExpandedNetwork: (name: string | null) => void;
  openNetworkForm: (index?: number) => void;
  closeNetworkForm: () => void;
  openServiceForm: (type?: "portproxy" | "socks5" | "rdp", index?: number) => void;
  closeServiceForm: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  currentPage: "main",
  expandedNetwork: null,
  networkFormOpen: false,
  serviceFormOpen: false,
  editingNetworkIndex: null,
  editingServiceType: null,
  editingServiceIndex: null,

  setCurrentPage: (page) => set({ currentPage: page }),
  setExpandedNetwork: (name) =>
    set((s) => ({ expandedNetwork: s.expandedNetwork === name ? null : name })),

  openNetworkForm: (index) =>
    set({
      networkFormOpen: true,
      editingNetworkIndex: index ?? null,
    }),
  closeNetworkForm: () =>
    set({ networkFormOpen: false, editingNetworkIndex: null }),

  openServiceForm: (type, index) =>
    set({
      serviceFormOpen: true,
      editingServiceType: type ?? null,
      editingServiceIndex: index ?? null,
    }),
  closeServiceForm: () =>
    set({
      serviceFormOpen: false,
      editingServiceType: null,
      editingServiceIndex: null,
    }),
}));
