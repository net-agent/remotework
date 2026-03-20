import { useCallback } from "react";
import { toast } from "sonner";
import { useSidecar } from "@/hooks/use-sidecar";
import { removeTunnelConfig } from "@/lib/tunnel-save";
import { useProfileStore } from "@/stores/profile-store";
import { useUIStore, type SimpleShareDialogType } from "@/stores/ui-store";

function useSimpleRuntimeActions() {
  const { startAgent, restartAgent } = useSidecar();
  const { activeProfile, clearNeedsRestart } = useProfileStore();

  const start = useCallback(async () => {
    if (!activeProfile) {
      toast.error("请先选择一个配置方案");
      return;
    }

    try {
      await startAgent(activeProfile);
      toast.success("已启动");
    } catch (error) {
      toast.error(`启动失败: ${error}`);
    }
  }, [activeProfile, startAgent]);

  const restart = useCallback(async () => {
    if (!activeProfile) {
      toast.error("请先选择一个配置方案");
      return;
    }

    try {
      await restartAgent(activeProfile);
      clearNeedsRestart();
      toast.success("已按新配置重启");
    } catch (error) {
      toast.error(`重启失败: ${error}`);
    }
  }, [activeProfile, clearNeedsRestart, restartAgent]);

  return { start, restart };
}

function useSimpleNavigationActions() {
  const {
    setUIMode,
    setAdvancedTab,
    setSimpleTask,
    openLinkForm,
    openServiceForm,
    setSelectedSimpleShareLinkAlias,
    openSimpleShareDialog,
    closeSimpleShareDialog,
  } = useUIStore();

  const createShareLink = useCallback(() => {
    setSimpleTask("share");
    openLinkForm();
  }, [openLinkForm, setSimpleTask]);

  const editShareLink = useCallback(
    (alias: string) => {
      setSimpleTask("share");
      openLinkForm(alias);
    },
    [openLinkForm, setSimpleTask],
  );

  const selectShareLink = useCallback(
    (alias: string | null) => {
      setSimpleTask("share");
      setSelectedSimpleShareLinkAlias(alias);
    },
    [setSelectedSimpleShareLinkAlias, setSimpleTask],
  );

  const configureShare = createShareLink;

  const configureConnect = useCallback(() => {
    setSimpleTask("connect");
    openServiceForm();
  }, [openServiceForm, setSimpleTask]);

  const openSimpleShareServiceDialog = useCallback(
    (type: SimpleShareDialogType) => {
      setSimpleTask("share");
      openSimpleShareDialog(type);
    },
    [openSimpleShareDialog, setSimpleTask],
  );

  const closeSimpleShareServiceDialog = useCallback(() => {
    closeSimpleShareDialog();
  }, [closeSimpleShareDialog]);

  const openShareServiceForm = useCallback(() => {
    openServiceForm();
  }, [openServiceForm]);

  const openAdvancedNetworks = useCallback(() => {
    setUIMode("advanced");
    setAdvancedTab("networks");
  }, [setAdvancedTab, setUIMode]);

  const openAdvancedServices = useCallback(() => {
    setUIMode("advanced");
    setAdvancedTab("services");
  }, [setAdvancedTab, setUIMode]);

  const openAdvancedConfig = useCallback(() => {
    setUIMode("advanced");
    setAdvancedTab("config");
  }, [setAdvancedTab, setUIMode]);

  return {
    configureShare,
    createShareLink,
    editShareLink,
    selectShareLink,
    configureConnect,
    openSimpleShareServiceDialog,
    closeSimpleShareServiceDialog,
    openShareServiceForm,
    openAdvancedNetworks,
    openAdvancedServices,
    openAdvancedConfig,
  };
}

export function useSimpleActions() {
  const runtimeActions = useSimpleRuntimeActions();
  const navigationActions = useSimpleNavigationActions();
  const {
    activeProfile,
    currentConfig,
    saveConfig,
    setCurrentConfig,
    setNeedsRestart,
  } = useProfileStore();

  const closeSimpleShareService = useCallback(
    async (input: { tunnelId?: string; configIndex?: number }) => {
      const removed = await removeTunnelConfig({
        tunnelId: input.tunnelId,
        configIndex: input.configIndex,
        context: {
          currentConfig,
          setCurrentConfig,
          saveConfig,
          activeProfile,
          setNeedsRestart,
        },
      });

      return removed;
    },
    [
      activeProfile,
      currentConfig,
      saveConfig,
      setCurrentConfig,
      setNeedsRestart,
    ],
  );

  return {
    ...runtimeActions,
    ...navigationActions,
    closeSimpleShareService,
  };
}
