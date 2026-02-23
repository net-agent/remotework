import { create } from "zustand";

type Page = "main" | "settings";
type MainTab = "networks" | "services" | "logs";

interface UIState {
  currentPage: Page;
  activeTab: MainTab;
  selectedNetwork: string | null;
  selectedService: number | null;
  networkFormOpen: boolean;
  serviceFormOpen: boolean;
  editingNetworkIndex: number | null;
  editingServiceType: "portproxy" | "socks5" | "rdp" | null;
  editingServiceIndex: number | null;

  setCurrentPage: (page: Page) => void;
  setActiveTab: (tab: MainTab) => void;
  setSelectedNetwork: (name: string | null) => void;
  setSelectedService: (id: number | null) => void;
  openNetworkForm: (index?: number) => void;
  closeNetworkForm: () => void;
  openServiceForm: (type?: "portproxy" | "socks5" | "rdp", index?: number) => void;
  closeServiceForm: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  currentPage: "main",
  activeTab: "networks",
  selectedNetwork: null,
  selectedService: null,
  networkFormOpen: false,
  serviceFormOpen: false,
  editingNetworkIndex: null,
  editingServiceType: null,
  editingServiceIndex: null,

  setCurrentPage: (page) => set({ currentPage: page }),
  setActiveTab: (tab) => set({ activeTab: tab }),
  setSelectedNetwork: (name) => set({ selectedNetwork: name }),
  setSelectedService: (id) => set({ selectedService: id }),

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
