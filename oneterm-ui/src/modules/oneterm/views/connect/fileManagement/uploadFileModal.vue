<template>
  <a-modal
    :title="$t('oneterm.fileManagement.uploadFile')"
    :visible="visible"
    :width="600"
    :zIndex="1004"
    :footer="null"
    :bodyStyle="{ paddingTop: '0px' }"
    @cancel="handleCancel"
  >
    <div class="upload-type-radio">
      <span class="upload-type-radio-label">{{ $t('oneterm.fileManagement.uploadTypeLabel') }}: </span>
      <a-radio-group
        v-model="uploadType"
        :options="uploadTypeOptions"
      />
    </div>
    <a-upload-dragger
      ref="upload"
      multiple
      :directory="uploadType === 'folder'"
      :showUploadList="false"
      :customRequest="handleCustomRequest"
    >
      <ops-icon
        type="itsm-folder"
        class="upload-dragger-icon"
      />
      <p class="upload-dragger-tip">
        <span class="upload-dragger-tip-bold">{{ $t('oneterm.fileManagement.uploadFileTip1', { name: uploadType === 'file' ? $t('oneterm.fileManagement.file') : $t('oneterm.fileManagement.folder') }) }}</span>
        <span>{{ $t('oneterm.fileManagement.uploadFileTip2') }}</span>
      </p>
    </a-upload-dragger>

    <ops-table
      v-if="fileList.length"
      size="mini"
      max-height="200px"
      ref="opsTable"
      :data="fileList"
      class="upload-file-table"
      show-overflow
      :row-config="{ height: 48 }"
    >
      <vxe-column
        :title="$t('name')"
        field="fileName"
      ></vxe-column>
      <vxe-column
        :title="$t('oneterm.fileManagement.path')"
        field="path"
      ></vxe-column>
      <vxe-column
        :title="$t('oneterm.fileManagement.uploadProgress')"
        field="progress"
        width="230"
      >
        <template #default="{row}">
          <a-progress
            :percent="row.uploadProgress"
            :success-percent="row.transmissionProgress"
            :status="getProgressStatus(row.status)"
          >
            <div slot="format"></div>
          </a-progress>
          <div class="upload-file-table-progress">
            <template v-if="row.status === UPLOAD_STATUS.SUCCESS">
              <a-icon
                style="color: #00b42a"
                type="check-circle"
              />
              <span>{{ $t('oneterm.fileManagement.uploadSuccess') }}</span>
            </template>
            <template v-else-if="row.status === UPLOAD_STATUS.ERROR">
              <a-icon
                style="color: #fd4c6a"
                type="close-circle"
              />
              <span>{{ $t('oneterm.fileManagement.uploadFailed') }}</span>
            </template>
            <template v-else-if="row.status === UPLOAD_STATUS.UPLOADING">
              <span>{{ $t('oneterm.fileManagement.transferToServer') }}</span>
              <span>{{ row.uploadProgress }}%</span>
            </template>
            <template v-else-if="row.status === UPLOAD_STATUS.TRANSMITTING">
              <span>{{ $t('oneterm.fileManagement.transferToTargetMachine') }}</span>
              <span>{{ row.transmissionProgress }}%</span>
            </template>
          </div>
        </template>
      </vxe-column>
    </ops-table>
  </a-modal>
</template>

<script>
import { v4 as uuidv4 } from 'uuid'
import { getFileTransferProgressById } from '@/modules/oneterm/api/file.js'

export default {
  name: 'UploadFileModal',
  props: {
    sessionId: {
      type: String,
      default: ''
    },
    pathStr: {
      type: String,
      default: ''
    },
    connectType: {
      type: String,
      default: 'ssh'
    }
  },
  data() {
    return {
      visible: false,
      uploadType: 'file',
      uploadTypeOptions: [
        { label: this.$t('oneterm.fileManagement.file'), value: 'file' },
        { label: this.$t('oneterm.fileManagement.folder'), value: 'folder' },
      ],
      fileList: [],
      UPLOAD_STATUS: {
        UPLOADING: 'uploading',
        TRANSMITTING: 'transmitting',
        SUCCESS: 'success',
        ERROR: 'error'
      }
    }
  },
  watch: {
    fileList: {
      deep: true,
      immediate: true,
      handler(fileList) {
        let uploading = 0
        let success = 0
        let error = 0

        fileList.map((file) => {
          switch (file.status) {
            case this.UPLOAD_STATUS.UPLOADING:
            case this.UPLOAD_STATUS.TRANSMITTING:
              uploading++
              break
            case this.UPLOAD_STATUS.SUCCESS:
              success++
              break
            case this.UPLOAD_STATUS.ERROR:
              error++
              break
            default:
              break
          }
        })

        this.$emit('updateCountData', {
          uploading,
          success,
          error
        })
      },
    }
  },
  methods: {
    open() {
      this.visible = true
    },
    handleCancel() {
      this.visible = false
    },
    handleCustomRequest(data) {
      let path = this.pathStr
      if (data?.file?.webkitRelativePath) {
        const relativePathList = data.file.webkitRelativePath.split('/')
        relativePathList.pop()
        path += `/${relativePathList.join('/')}`
      }
      const file = {
        id: uuidv4(),
        file: data.file,
        fileName: data?.file?.name || '-',
        path,
        uploadProgress: 0,
        transmissionProgress: 0,
        status: this.UPLOAD_STATUS.UPLOADING,
        xhr: null,
        progressInterval: null
      }

      this.uploadFile(file)
      this.fileList.unshift(file)
    },
    uploadFile(file) {
      const xhr = new XMLHttpRequest()
      file.xhr = xhr

      xhr.upload.addEventListener('progress', (e) => {
        if (!e.lengthComputable) {
          return
        }

        if (e.loaded === e.total) {
          file.status = this.UPLOAD_STATUS.TRANSMITTING
          file.uploadProgress = 100
          file.progressInterval = setInterval(async () => {
            const res = await getFileTransferProgressById(
              file.id,
              {
                type: this.connectType === 'ssh' ? 'sftp' : 'rdp'
              }
            )

            if (file.status !== this.UPLOAD_STATUS.TRANSMITTING) {
              clearInterval(file.progressInterval)
              return
            }

            const progress = Math.round(res?.data?.progress || 0)
            if (progress === 100) {
              clearInterval(file.progressInterval)
            }
            file.transmissionProgress = progress
          }, 1000)
        } else {
          file.uploadProgress = Math.round((e.loaded / e.total) * 100)
        }
      })

      xhr.onreadystatechange = (data) => {
        if (xhr.readyState !== 4) {
          return
        }

        if (file.progressInterval) {
          clearInterval(file.progressInterval)
        }

        if (xhr.status >= 200 && xhr.status < 300) {
          file.uploadProgress = 100
          file.transmissionProgress = 100
          file.status = this.UPLOAD_STATUS.SUCCESS
          this.$emit('uploadSuccess', file)
        } else if (xhr.status >= 400) {
          file.status = this.UPLOAD_STATUS.ERROR

          let errorMessage = this.$t('oneterm.fileManagement.uploadFailed')
          if (xhr.responseText) {
            const responseText = JSON.parse(xhr.responseText)
            if (responseText?.message) {
              errorMessage = responseText.message
            }
          }
          this.$message.error(errorMessage)
        }
      }

      let postURL = `/api/oneterm/v1/file/session/${this.sessionId}/upload?dir=${file.path}&transfer_id=${file.id}`

      if (this.connectType === 'rdp') {
        postURL = `/api/oneterm/v1/rdp/sessions/${this.sessionId}/files/upload?path=${file.path}&transfer_id=${file.id}`
      }

      const formData = new FormData()
      formData.append('file', file.file)

      xhr.open('POST', postURL)
      xhr.send(formData)
    },

    getProgressStatus(status) {
      switch (status) {
        case this.UPLOAD_STATUS.uploading:
          return 'normal'
        case this.UPLOAD_STATUS.SUCCESS:
          return 'success'
        case this.UPLOAD_STATUS.ERROR:
          return 'exception'
        default:
          return 'normal'
      }
    }
  }
}
</script>

<style lang="less" scoped>
.upload-type-radio {
  margin-bottom: 16px;

  &-label {
    margin-right: 6px;
  }
}
.upload-dragger-icon {
  font-size: 58px;
}
.upload-dragger-tip {
  margin-top: 8px;

  &-bold {
    color: @primary-color;
    font-weight: 600;
  }
}
.upload-file-table {
  margin-top: 16px;

  &-progress {
    display: flex;
    width: 100%;
    align-items: center;
    column-gap: 3px;
    line-height: 12px;

    & > span {
      font-size: 12px;
      color: #606266;
    }
  }
}
</style>
