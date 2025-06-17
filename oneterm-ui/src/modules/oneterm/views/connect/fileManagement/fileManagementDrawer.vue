<template>
  <CustomDrawer
    width="700px"
    :visible="visible"
    :zIndex="1003"
    :maskClosable="false"
    @close="handleClose"
  >
    <div
      slot="title"
      class="file-management-title"
    >
      <div class="file-management-title-name">
        {{ $t('oneterm.fileManagement.name') }}
      </div>
      <div class="file-management-title-right">
        <a-tooltip
          v-if="selectedRows.length"
          :title="$t('oneterm.fileManagement.batchDownloadFiles')"
        >
          <a-icon
            @click="batchDownloadFiles"
            type="download"
          />
        </a-tooltip>
        <UploadFile
          :sessionId="sessionId"
          :pathStr="pathStr"
          :connectType="connectType"
          @updateFileList="getFileList()"
        />
      </div>
    </div>
    <a-spin :spinning="loading">
      <a-space class="file-management-header">
        <a-input
          v-model="pathInputValue"
          :placeholder="$t('oneterm.fileManagement.directoryInputPlaceholder')"
          class="file-management-input"
          @pressEnter="handleInputPressEnter"
        />
        <a-tooltip :title="$t('oneterm.fileManagement.backToPreviousLevel')">
          <div
            :class="['file-management-action', pathList.length === 0 ? 'file-management-action_disabled' : '']"
            @click="clickPathBack"
          >
            <a-icon type="rollback" />
          </div>
        </a-tooltip>
        <a-tooltip
          v-if="connectType === 'ssh'"
          :title="$t('refresh')"
        >
          <div
            class="file-management-action"
            @click="getFileList()"
          >
            <ops-icon type="veops-refresh" />
          </div>
        </a-tooltip>
        <a-tooltip v-if="!showHiddenFiles" :title="$t('oneterm.fileManagement.showHiddenFiles')">
          <div
            class="file-management-action"
            @click="toggleHiddenFileDisplay"
          >
            <a-icon type="eye" />
          </div>
        </a-tooltip>
        <a-tooltip v-else :title="$t('oneterm.fileManagement.hideHiddenFiles')">
          <div
            class="file-management-action"
            @click="toggleHiddenFileDisplay"
          >
            <a-icon type="eye-invisible" />
          </div>
        </a-tooltip>
      </a-space>

      <ops-table
        size="mini"
        :height="`${windowHeight - 205}px`"
        ref="opsTable"
        :checkbox-config="{ highlight: true, range: true, checkMethod: handleCheckboxDisabled }"
        :data="tableData"
        highlight-hover-row
        @checkbox-change="onSelectChange"
        @checkbox-all="onSelectChange"
        @checkbox-range-end="onSelectChange"
      >
        <vxe-column type="checkbox" width="40px"></vxe-column>
        <vxe-column
          :title="$t('name')"
          field="name"
          sortable
        >
          <template #default="{row}">
            <div
              :class="['file-management-table-path', row.is_dir ? 'file-management-table-path_folder' : '']"
              @click="clickPath(row)"
            >
              <ops-icon
                :type="row.icon"
                class="file-management-table-path-icon"
              />
              <a-tooltip :title="row.nameText">
                <span class="file-management-table-path-text">{{ row.nameText }}</span>
              </a-tooltip>
            </div>
          </template>
        </vxe-column>
        <vxe-column
          :title="$t('oneterm.fileManagement.size')"
          field="size"
          sortable
          width="100"
        >
          <template #default="{row}">
            <span>{{ row.sizeText }}</span>
          </template>
        </vxe-column>
        <vxe-column
          :title="$t('oneterm.fileManagement.lastModified')"
          field="mod_time"
          sortable
          width="160"
        >
          <template #default="{row}">
            <span>{{ row.lastModified }}</span>
          </template>
        </vxe-column>
        <vxe-column
          v-if="connectType === 'ssh'"
          :title="$t('oneterm.fileManagement.permissions')"
          field="mode"
          width="100"
        ></vxe-column>
      </ops-table>
    </a-spin>
  </CustomDrawer>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import { getFileListBySessionId } from '@/modules/oneterm/api/file.js'
import { getRDPFileList } from '@/modules/oneterm/api/rdp.js'

import UploadFile from './uploadFile.vue'

export default {
  name: 'FileManagementDrawer',
  components: {
    UploadFile
  },
  props: {
    sessionId: {
      type: String,
      default: ''
    },
    connectType: {
      type: String,
      default: 'ssh' // 'ssh' | 'rdp'
    }
  },
  data() {
    return {
      visible: false,
      pathInputValue: '',
      loading: false,
      tableData: [],
      selectedRows: [],
      pathList: [],
      showHiddenFiles: false,
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
    }),
    pathStr() {
      return `/${this.pathList.join('/')}`
    }
  },
  methods: {
    open() {
      this.visible = true
      this.getFileList([])
    },
    getFileList(pathList) {
      this.selectedRows = []
      this.loading = true
      const dir = `/${(pathList || this.pathList).join('/')}`

      ;(this.connectType === 'ssh'
        ? this.getSSHFileListData(dir)
        : this.getRDPFileListData(dir)
      ).then((tableData) => {
        this.tableData = tableData
        this.pathInputValue = dir
        if (pathList) {
          this.pathList = pathList
        }
      }).finally(() => {
        this.loading = false
      })
    },

    async getSSHFileListData(dir) {
      const res = await getFileListBySessionId(
        this.sessionId,
        {
          dir,
          show_hidden: this.showHiddenFiles
        }
      )
      const tableData = res?.data?.list || []
      this.handleTableData(tableData)

      return tableData
    },

    async getRDPFileListData(dir) {
      const res = await getRDPFileList(
        this.sessionId,
        {
          path: dir
        }
      )
      const tableData = res?.data || []
      this.handleTableData(tableData)

      return tableData
    },

    handleTableData(tableData) {
      tableData.forEach((row) => {
        row.sizeText = row.is_dir ? '-' : this.getSizeText(row.size)
        row.lastModified = row.mod_time ? moment(row.mod_time).format('YYYY-MM-DD HH:mm:ss') : '-'

        row.icon = 'file'
        if (row.is_dir) {
          row.icon = 'folder1'
        } else if (row.is_link) {
          row.icon = 'onterm-symbolic_link'
        }

        row.nameText = row.name
        if (row.is_link && row.target) {
          row.nameText = `${row.name} -> ${row.target}`
        }
      })
      return tableData
    },

    getSizeText(value, divideNum = 1024) {
      if (value < divideNum ** 1) return `${value.toFixed(1)}B`
      else if (value < divideNum ** 2) return `${(value / divideNum ** 1).toFixed(1)}KB`
      else if (value < divideNum ** 3) return `${(value / divideNum ** 2).toFixed(1)}MB`
      else if (value < divideNum ** 4) return `${(value / divideNum ** 3).toFixed(1)}GB`
      else if (value < divideNum ** 5) return `${(value / divideNum ** 4).toFixed(1)}TB`
      else return `${(value / divideNum ** 5).toFixed(1)}PB`
    },

    handleCheckboxDisabled({ row }) {
      return !row?.is_link
    },

    onSelectChange({ records }) {
      this.selectedRows = records
    },

    clickPath(row) {
      if (!row?.is_dir) {
        return
      }

      const pathList = [...this.pathList]
      pathList.push(row.name)
      this.getFileList(pathList)
    },
    clickPathBack() {
      if (!this.pathList.length) {
        return
      }

      this.pathList.pop()
      this.getFileList()
    },
    toggleHiddenFileDisplay() {
      this.showHiddenFiles = !this.showHiddenFiles
      this.getFileList()
    },
    handleInputPressEnter() {
      const pathInputValue = this.pathInputValue.startsWith('/') ? this.pathInputValue.slice(1) : this.pathInputValue
      const pathList = pathInputValue.split('/')
      this.getFileList(pathList)
    },
    async batchDownloadFiles() {
      if (!this.selectedRows.length) {
        return
      }

      const a = document.createElement('a')
      a.target = '_blank'

      const names = this.selectedRows.map((row) => row.name).join(',')
      let href = `${document.location.origin}/api/oneterm/v1/file/session/${this.sessionId}/download?dir=${this.pathStr}&names=${names}`
      if (this.connectType === 'rdp') {
        href = `${document.location.origin}/api/oneterm/v1/rdp/sessions/${this.sessionId}/files/download?dir=${this.pathStr}&names=${names}`
      }
      a.href = href

      document.body.appendChild(a)
      a.click()
      a.remove()
      this.selectedRows = []
    },
    handleClose() {
      this.visible = false
      this.tableData = []
      this.selectedRows = []
      this.pathList = []
      this.pathInputValue = ''
      this.loading = false
    }
  }
}
</script>

<style lang="less" scoped>
.file-management {
  &-title {
    display: flex;
    justify-content: space-between;

    &-right {
      display: flex;
      column-gap: 8px;
      align-items: center;
    }
  }

  &-header {
    width: 100%;
    margin-bottom: 16px;

    /deep/ .ant-space-item:first-child {
      flex-grow: 1;
    }
  }

  &-action {
    height: 32px;
    width: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid #e4e7ed;
    cursor: pointer;
    border-radius: 2px;

    &_disabled {
      background-color: #f5f5f5;
      color: rgba(0, 0, 0, .25) !important;
      cursor: not-allowed;
    }

    &:hover {
      color: @primary-color;
    }
  }

  &-table-path {
    display: flex;
    align-items: center;

    &-icon {
      font-size: 16px;
      margin-right: 4px;
      color: #bdd3f4;
    }

    &-text {
      font-weight: 600;
      overflow: hidden;
      text-overflow: ellipsis;
      text-wrap: nowrap;
    }

    &_folder {
      cursor: pointer;

      &:hover {
        .file-management-table-path-text {
          color: @primary-color;
        }
      }
    }
  }
}
</style>
