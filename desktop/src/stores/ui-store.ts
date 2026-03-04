import { create } from "zustand";

type Page = "main" | "settings";
type MainTab = "networks" | "services" | "logs";

interface UIState {
  currentPage: Page;
  activeTab: MainTab;
  selectedNetwork: string | null;
  selectedService: number | null;
  linkFormOpen: boolean;
  serviceFormOpen: boolean;
  editingLinkAlias: string | null;
  editingServiceIndex: number | null;

  setCurrentPage: (page: Page) => void;
  setActiveTab: (tab: MainTab) => void;
  setSelectedNetwork: (name: string | null) => void;
  setSelectedService: (id: number | null) => void;
  openLinkForm: (alias?: string) => void;
  closeLinkForm: () => void;
  openServiceForm: (index?: number) => void;
  closeServiceForm: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  currentPage: "main",
  activeTab: "networks",
  selectedNetwork: null,
  selectedService: null,
  linkFormOpen: false,
  serviceFormOpen: false,
  editingLinkAlias: null,
  editingServiceIndex: null,

  setCurrentPage: (page) => set({ currentPage: page }),
  setActiveTab: (tab) => set({ activeTab: tab }),
  setSelectedNetwork: (name) => set({ selectedNetwork: name }),
  setSelectedService: (id) => set({ selectedService: id }),

  openLinkForm: (alias) =>
    set({
      linkFormOpen: true,
      editingLinkAlias: alias ?? null,
    }),
  closeLinkForm: () => set({ linkFormOpen: false, editingLinkAlias: null }),

  openServiceForm: (index) =>
    set({
      serviceFormOpen: true,
      editingServiceIndex: index ?? null,
    }),
  closeServiceForm: () =>
    set({
      serviceFormOpen: false,
      editingServiceIndex: null,
    }),
}));
