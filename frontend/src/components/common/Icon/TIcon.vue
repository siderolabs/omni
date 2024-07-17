<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <img v-if="svgBase64" alt="" :src="`data:image/svg+xml;base64,${svgBase64}`"/>
  <component v-else :is="component" class="min-w-content"/>
</template>

<script setup lang="ts">
import { type Component, computed, defineAsyncComponent, toRefs } from "vue";
import {
  UsersIcon,
  DocumentIcon,
  PowerIcon,
  UserIcon,
  UserPlusIcon,
  TagIcon,
  ArrowUpCircleIcon,
  ArrowUpTrayIcon,
  DocumentTextIcon,
  LockClosedIcon,
  LockOpenIcon,
  WindowIcon,
  CodeBracketIcon,
  PlayCircleIcon,
  CircleStackIcon,
  ChartBarIcon,
  ServerIcon,
  ServerStackIcon,
} from "@heroicons/vue/24/outline";

const icons = {
  "action-horizontal": defineAsyncComponent(() => import("../../icons/IconActionHorizontal.vue")),
  "dropdown": defineAsyncComponent(() => import("../../icons/IconDropdown.vue")),
  "cloud-connection": defineAsyncComponent(() => import("../../icons/IconCloudConnection.vue")),
  "arrow-right-square": defineAsyncComponent(() => import("../../icons/IconArrowRightSquare.vue")),
  "box": defineAsyncComponent(() => import("../../icons/IconBox.vue")),
  "minus": defineAsyncComponent(() => import("../../icons/IconMinus.vue")),
  "plus": defineAsyncComponent(() => import("../../icons/IconPlus.vue")),
  "loading": defineAsyncComponent(() => import("../../icons/IconLoading.vue")),
  "check-in-circle": defineAsyncComponent(() => import("../../icons/IconCheckInCircle.vue")),
  "upload": defineAsyncComponent(() => import("../../icons/IconUpload.vue")),
  "arrow-down": defineAsyncComponent(() => import("../../icons/IconArrowDown.vue")),
  "arrow-up": defineAsyncComponent(() => import("../../icons/IconArrowUp.vue")),
  "arrow-right": defineAsyncComponent(() => import("../../icons/IconArrowRight.vue")),
  "long-arrow-top": defineAsyncComponent(() => import("../../icons/IconLongArrowTop.vue")),
  "long-arrow-down": defineAsyncComponent(() => import("../../icons/IconLongArrowDown.vue")),
  "copy": defineAsyncComponent(() => import("../../icons/IconCopy.vue")),
  "delete": defineAsyncComponent(() => import("../../icons/IconDelete.vue")),
  "settings": defineAsyncComponent(() => import("../../icons/IconSettings.vue")),
  "close": defineAsyncComponent(() => import("../../icons/IconClose.vue")),
  "error": defineAsyncComponent(() => import("../../icons/IconError.vue")),
  "info": defineAsyncComponent(() => import("../../icons/IconInfo.vue")),
  "check-in-circle-classic": defineAsyncComponent(() => import("../../icons/IconCheckInCircleClassic.vue")),
  "overview": defineAsyncComponent(() => import("../../icons/IconOverview.vue")),
  "nodes": defineAsyncComponent(() => import("../../icons/IconNodes.vue")),
  "podes": defineAsyncComponent(() => import("../../icons/IconPodes.vue")),
  "clusters": defineAsyncComponent(() => import("../../icons/IconClusters.vue")),
  "clusters-big": defineAsyncComponent(() => import("../../icons/IconClustersBig.vue")),
  "dot": defineAsyncComponent(() => import("../../icons/IconDot.vue")),
  "time": defineAsyncComponent(() => import("../../icons/IconTime.vue")),
  "drop-up": defineAsyncComponent(() => import("../../icons/IconDropUp.vue")),
  "search": defineAsyncComponent(() => import("../../icons/IconSearch.vue")),
  "pin": defineAsyncComponent(() => import("../../icons/IconPin.vue")),
  "arrow-left": defineAsyncComponent(() => import("../../icons/IconArrowLeft.vue")),
  "check": defineAsyncComponent(() => import("../../icons/IconCheck.vue")),
  "kube-config": defineAsyncComponent(() => import("../../icons/IconKubeConfig.vue")),
  "talos-config": defineAsyncComponent(() => import("../../icons/IconTalosConfig.vue")),
  "kubernetes": defineAsyncComponent(() => import("../../icons/IconKubernetes.vue")),
  "edit": defineAsyncComponent(() => import("../../icons/IconEdit.vue")),
  "unlink": defineAsyncComponent(() => import("../../icons/IconUnlink.vue")),
  "refresh": defineAsyncComponent(() => import("../../icons/IconRefresh.vue")),
  "long-arrow-left": defineAsyncComponent(() => import("../../icons/IconLongArrowLeft.vue")),
  "long-arrow-right": defineAsyncComponent(() => import("../../icons/IconLongArrowRight.vue")),
  "warning-clear": defineAsyncComponent(() => import("../../icons/IconWarningClear.vue")),
  "warning": defineAsyncComponent(() => import("../../icons/IconWarning.vue")),
  "dashboard": defineAsyncComponent(() => import("../../icons/IconDashboard.vue")),
  "reset": defineAsyncComponent(() => import("../../icons/IconReset.vue")),
  "reboot": defineAsyncComponent(() => import("../../icons/IconReboot.vue")),
  "change": defineAsyncComponent(() => import("../../icons/IconChange.vue")),
  "action-vertical": defineAsyncComponent(() => import("../../icons/IconActionVertical.vue")),
  "ongoing-tasks": defineAsyncComponent(() => import("../../icons/IconOngoingTasks.vue")),
  "header-logo": defineAsyncComponent(() => import("../../icons/IconHeaderLogo.vue")),
  "logo": defineAsyncComponent(() => import("../../icons/IconLogo.vue")),
  "home": defineAsyncComponent(() => import("../../icons/IconHome.vue")),
  "attention": defineAsyncComponent(() => import("../../icons/IconAttention.vue")),
  "waiting": defineAsyncComponent(() => import("../../icons/IconWaiting.vue")),
  "complete": defineAsyncComponent(() => import("../../icons/IconComplete.vue")),
  "in-progress": defineAsyncComponent(() => import("../../icons/IconInProgress.vue")),
  "drop-right": defineAsyncComponent(() => import("../../icons/IconDropRight.vue")),
  "log": defineAsyncComponent(() => import("../../icons/IconLog.vue")),
  "external-link": defineAsyncComponent(() => import("../../icons/IconExternalLink.vue")),
  "no-connection": defineAsyncComponent(() => import("../../icons/IconNoConnection.vue")),
  "fail-auth": defineAsyncComponent(() => import("../../icons/IconFailAuth.vue")),
  "key": defineAsyncComponent(() => import("../../icons/IconKey.vue")),
  "aws": defineAsyncComponent(() => import("../../icons/IconAWS.vue")),
  "gcp": defineAsyncComponent(() => import("../../icons/IconGCP.vue")),
  "sidero": defineAsyncComponent(() => import("../../icons/IconSidero.vue")),
  "sidero-monochrome": defineAsyncComponent(() => import("../../icons/IconSideroMonochrome.vue")),
  "terminal": defineAsyncComponent(() => import("../../icons/IconTerminal.vue")),
  "unknown": defineAsyncComponent(() => import("../../icons/IconUnknown.vue")),
  "question": defineAsyncComponent(() => import("../../icons/IconQuestion.vue")),
  "upgrade-empty-state": defineAsyncComponent(() => import("../../icons/IconUpgradeEmptyState.vue")),
  "link-down": defineAsyncComponent(() => import("../../icons/IconLinkDown.vue")),
  "settings-toggle": defineAsyncComponent(() => import("../../icons/IconSettingsToggle.vue")),
  "rollback": defineAsyncComponent(() => import("../../icons/IconRollback.vue")),
  "extensions": defineAsyncComponent(() => import("../../icons/IconExtensions.vue")),
  "extensions-toggle": defineAsyncComponent(() => import("../../icons/IconExtensionsToggle.vue")),
  "document": DocumentIcon,
  "power": PowerIcon,
  "users": UsersIcon,
  "user": UserIcon,
  "user-add": UserPlusIcon,
  "tag": TagIcon,
  "arrow-up-circle": ArrowUpCircleIcon,
  "arrow-up-tray": ArrowUpTrayIcon,
  "bootstrap-manifests": DocumentTextIcon,
  "exposed-service": WindowIcon,
  "locked": LockClosedIcon,
  "locked-toggle": defineAsyncComponent(() => import("../../icons/IconLockClosedToggle.vue")),
  "unlocked": LockOpenIcon,
  "code-bracket": CodeBracketIcon,
  "play-circle": PlayCircleIcon,
  "circle-stack": CircleStackIcon,
  "chart-bar": ChartBarIcon,
  "server": ServerIcon,
  "server-stack": ServerStackIcon,
};

const getComponent = (icon: string): Component | undefined => {
  return icons[icon];
}

export type IconType = keyof typeof icons;

type Props = {
  svgBase64?: string
  icon?: IconType;
}

const props = withDefaults(defineProps<Props>(), {
  icon: "action-horizontal",
});

const { icon } = toRefs(props);

const component = computed(() => {
  return getComponent(icon.value);
});
</script>
