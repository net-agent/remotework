import { create } from "zustand";

type Page = "main" | "settings";
type UIMode = "simple" | "advanced";
type SimpleTask = "share" | "connect";
type AdvancedTab = "networks" | "services" | "logs" | "config";
export type SimpleShareDialogType = "socks5" | "local-port";

interface UIState {
  currentPage: Page;
  uiMode: UIMode;
  simpleTask: SimpleTask;
  advancedTab: AdvancedTab;
  selectedNetwork: string | null;
  selectedService: number | null;
  selectedSimpleShareLinkAlias: string | null;
  simpleShareDialogType: SimpleShareDialogType | null;
  simpleShareDialogOpen: boolean;
  linkFormOpen: boolean;
  serviceFormOpen: boolean;
  editingLinkAlias: string | null;
  editingServiceIndex: number | null;

  setCurrentPage: (page: Page) => void;
  setUIMode: (mode: UIMode) => void;
  setSimpleTask: (task: SimpleTask) => void;
  setAdvancedTab: (tab: AdvancedTab) => void;
  setSelectedNetwork: (name: string | null) => void;
  setSelectedService: (id: number | null) => void;
  setSelectedSimpleShareLinkAlias: (alias: string | null) => void;
  openSimpleShareDialog: (type: SimpleShareDialogType) => void;
  closeSimpleShareDialog: () => void;
  openLinkForm: (alias?: string) => void;
  closeLinkForm: () => void;
  openServiceForm: (index?: number) => void;
  closeServiceForm: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  currentPage: "main",
  uiMode: "simple",
  simpleTask: "share",
  advancedTab: "networks",
  selectedNetwork: null,
  selectedService: null,
  selectedSimpleShareLinkAlias: null,
  simpleShareDialogType: null,
  simpleShareDialogOpen: false,
  linkFormOpen: false,
  serviceFormOpen: false,
  editingLinkAlias: null,
  editingServiceIndex: null,

  setCurrentPage: (page) => set({ currentPage: page }),
  setUIMode: (mode) =>
    set({
      uiMode: mode,
      ...(mode === "advanced"
        ? {
            simpleShareDialogType: null,
            simpleShareDialogOpen: false,
          }
        : {}),
    }),
  setSimpleTask: (task) =>
    set({
      simpleTask: task,
      ...(task === "share"
        ? {}
        : {
            simpleShareDialogType: null,
            simpleShareDialogOpen: false,
          }),
    }),
  setAdvancedTab: (tab) => set({ advancedTab: tab }),
  setSelectedNetwork: (name) => set({ selectedNetwork: name }),
  setSelectedService: (id) => set({ selectedService: id }),
  setSelectedSimpleShareLinkAlias: (alias) =>
    set({
      selectedSimpleShareLinkAlias: alias,
      simpleShareDialogType: null,
      simpleShareDialogOpen: false,
    }),
  openSimpleShareDialog: (type) =>
    set({
      simpleShareDialogType: type,
      simpleShareDialogOpen: true,
    }),
  closeSimpleShareDialog: () =>
    set({
      simpleShareDialogOpen: false,
      simpleShareDialogType: null,
    }),

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
