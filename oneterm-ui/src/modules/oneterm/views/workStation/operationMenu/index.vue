<template>
  <div class="workstation-operation-menu">
    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.FULL_SCREEN)"
      :title="openFullScreen ? $t('oneterm.workStation.exitFullScreen') : $t('oneterm.workStation.fullScreen')"
      placement="left"
    >
      <a-icon
        :type="openFullScreen ? 'fullscreen-exit' : 'fullscreen'"
        @click="toggleFullScreen"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.RECENT_SESSION)"
      :title="$t('oneterm.workStation.recentSession')"
      placement="left"
    >
      <ops-icon
        type="ops-oneterm-sessionhistory"
        @click="openRecentSession"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.BATCH_EXECUTION)"
      :title="$t('oneterm.workStation.batchExecution')"
      placement="left"
    >
      <ops-icon
        type="oneterm-batch_execution"
        @click="openChooseAssetsModal"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.DISPLAY_SETTING)"
      :title="$t('oneterm.terminalDisplay.displaySetting')"
      placement="left"
    >
      <ops-icon
        type="terminal_settings"
        @click="openSystemSetting(WORKSTATION_TAB_TYPE.DISPLAY_SETTING)"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.THEME_SETTING)"
      :title="$t('oneterm.terminalDisplay.themeSetting')"
      placement="left"
    >
      <ops-icon
        type="ops-setting-theme"
        @click="openSystemSetting(WORKSTATION_TAB_TYPE.THEME_SETTING)"
      />
    </a-tooltip>

    <a-divider
      v-if="showDivider"
      class="workstation-operation-menu-divider"
    ></a-divider>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.SHARE)"
      :title="$t('oneterm.workStation.assetShare')"
      placement="left"
    >
      <ops-icon
        type="veops-share"
        @click="shareAsset"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.QUICK_COMMAND)"
      :title="$t('oneterm.quickCommand.name')"
      placement="left"
    >
      <ops-icon
        type="quick_commands"
        @click="callComponentFn('openCommandDrawer')"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.FILE_MANAGEMENT)"
      :title="$t('oneterm.fileManagement.name')"
      placement="left"
    >
      <a-icon
        type="folder"
        @click="callComponentFn('openFileManagementDrawer')"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.CLIPBOARD)"
      :title="$t('oneterm.guacamole.clipboard')"
      placement="left"
    >
      <a-icon
        type="copy"
        @click="callComponentFn('openClipboardModal')"
      />
    </a-tooltip>

    <a-tooltip
      v-if="controlDisplayList.includes(OPERATION_MENU_TYPE.RESOLUTION)"
      :title="$t('oneterm.terminalDisplay.resolution')"
      placement="left"
    >
      <a-icon
        type="desktop"
        @click="callComponentFn('openResolutionModal')"
      />
    </a-tooltip>

    <ChooseAssetsModal
      ref="chooseAssetsModalRef"
      :accountList="accountList"
      @ok="openBatchExecution"
    />
    <ShareAssetModal
      ref="shareAssetModalRef"
      :assetData="currentTabData"
    />
  </div>
</template>

<script>
import { WORKSTATION_TAB_TYPE, SOCKET_STATUS } from '@/modules/oneterm/views/workStation/constants.js'
import { OPERATION_MENU_TYPE } from './constants.js'
import { PERMISSION_TYPE } from '@/modules/oneterm/views/systemSettings/accessControl/constants.js'

import ChooseAssetsModal from '../batchExecution/chooseAssetsModal.vue'
import ShareAssetModal from './shareAssetModal.vue'

export default {
  name: 'OperationMenu',
  components: {
    ChooseAssetsModal,
    ShareAssetModal
  },
  props: {
    openFullScreen: {
      type: Boolean,
      default: false
    },
    accountList: {
      type: Array,
      default: () => []
    },
    currentTabData: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      WORKSTATION_TAB_TYPE,
      OPERATION_MENU_TYPE
    }
  },
  computed: {
    isTerminal() {
      return this.currentTabData?.type === WORKSTATION_TAB_TYPE.TERMINAL
    },
    isGuacamole() {
      return this.currentTabData?.type === WORKSTATION_TAB_TYPE.GUACAMOLE
    },
    controlDisplayList() {
      const assetPermissions = this.currentTabData?.permissions || {}
      const socketStatus = this.currentTabData?.socketStatus || SOCKET_STATUS.LOADING

      const controlDisplayList = [
        OPERATION_MENU_TYPE.FULL_SCREEN,
        OPERATION_MENU_TYPE.RECENT_SESSION,
        OPERATION_MENU_TYPE.BATCH_EXECUTION,
        OPERATION_MENU_TYPE.DISPLAY_SETTING,
        OPERATION_MENU_TYPE.THEME_SETTING
      ]

      if (this.isGuacamole && socketStatus === SOCKET_STATUS.SUCCESS) {
        controlDisplayList.push(OPERATION_MENU_TYPE.RESOLUTION)
      }

      const showShare = assetPermissions?.[PERMISSION_TYPE.SHARE] || false
      if (showShare && (this.isGuacamole || this.isTerminal)) {
        controlDisplayList.push(OPERATION_MENU_TYPE.SHARE)
      }

      if (this.isTerminal && socketStatus === SOCKET_STATUS.SUCCESS) {
        controlDisplayList.push(OPERATION_MENU_TYPE.QUICK_COMMAND)
      }

      const showUpload = assetPermissions?.[PERMISSION_TYPE.FILE_UPLOAD] || false
      const showDownload = assetPermissions?.[PERMISSION_TYPE.FILE_DOWNLOAD] || false
      if (
        ['ssh', 'rdp'].includes(this.currentTabData?.protocolType) &&
        (showUpload || showDownload) &&
        socketStatus === SOCKET_STATUS.SUCCESS
      ) {
        controlDisplayList.push(OPERATION_MENU_TYPE.FILE_MANAGEMENT)
      }

      const showClipboard = assetPermissions?.[PERMISSION_TYPE.PASTE] || false
      if (
        this.isGuacamole &&
        showClipboard &&
        socketStatus === SOCKET_STATUS.SUCCESS
      ) {
        controlDisplayList.push(OPERATION_MENU_TYPE.CLIPBOARD)
      }

      return controlDisplayList
    },
    showDivider() {
      return this.controlDisplayList.some((item) => {
        return [
          OPERATION_MENU_TYPE.QUICK_COMMAND,
          OPERATION_MENU_TYPE.FILE_MANAGEMENT,
          OPERATION_MENU_TYPE.CLIPBOARD,
          OPERATION_MENU_TYPE.RESOLUTION
        ].includes(item)
      })
    }
  },
  methods: {
    toggleFullScreen() {
      this.$emit('toggleFullScreen')
    },
    openRecentSession() {
      this.$emit('openRecentSession')
    },
    openChooseAssetsModal() {
      this.$refs.chooseAssetsModalRef.open()
    },
    openBatchExecution(data) {
      this.$emit('openBatchExecution', data)
    },
    openSystemSetting(type) {
      this.$emit('openSystemSetting', type)
    },
    callComponentFn(name) {
      this.$emit('callComponentFn', name)
    },
    shareAsset() {
      this.$refs.shareAssetModalRef.open()
    }
  }
}
</script>

<style lang="less" scoped>
.workstation-operation-menu {
  display: flex;
  flex-direction: column;
  row-gap: 18px;
  background-color: #EBEBEB90;
  padding: 14px 0px;
  transition: width 0.1s;
  overflow: hidden;
  flex-shrink: 0;

  & > i {
    font-size: 18px;
  }

  &-divider {
    margin: 0px;
  }
}
</style>
