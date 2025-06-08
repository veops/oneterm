<template>
  <div
    @click="uploadFile"
    :class="['upload-file', showCountData ? 'upload-file_border' : '']"
  >
    <a-tooltip :title="$t('oneterm.fileManagement.uploadFile')">
      <a-icon
        type="upload"
      />
    </a-tooltip>
    <div
      v-if="showCountData"
      class="upload-file-right"
    >
      <template v-if="countData.uploading">
        <span class="upload-file-divider">|</span>
        <a-icon type="loading" /> {{ countData.uploading }}
      </template>
      <span class="upload-file-divider">|</span>
      <a-icon style="color: #00b42a" type="check-circle" /> {{ countData.success }}
      <span class="upload-file-divider">|</span>
      <a-icon style="color: #fd4c6a" type="close-circle" /> {{ countData.error }}
    </div>

    <UploadFileModal
      ref="uploadFileModalRef"
      :sessionId="sessionId"
      :pathStr="pathStr"
      :connectType="connectType"
      @updateCountData="updateCountData"
      @uploadSuccess="uploadSuccess"
    />
  </div>
</template>

<script>
import _ from 'lodash'

import UploadFileModal from './uploadFileModal.vue'

export default {
  name: 'UploadFile',
  components: {
    UploadFileModal
  },
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
      countData: {
        uploading: 0,
        success: 0,
        error: 0
      }
    }
  },
  computed: {
    showCountData() {
      const countData = this.countData
      return countData.uploading !== 0 || countData.success !== 0 || countData.error !== 0
    }
  },
  methods: {
    uploadFile() {
      this.$refs.uploadFileModalRef.open()
    },
    updateCountData(data) {
      this.countData = {
        uploading: data.uploading || 0,
        success: data.success || 0,
        error: data.error || 0
      }
    },
    uploadSuccess(file) {
      if (file.path === this.pathStr) {
        this.updateFileList()
      }
    },
    updateFileList: _.throttle(
      function() {
        this.$emit('updateFileList')
      },
      1000,
      {
        leading: false,
        trailing: true,
      }
    )
  }
}
</script>

<style lang="less" scoped>
.upload-file {
  display: flex;
  align-items: center;
  padding: 2px 8px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 400;

  &_border {
    border: solid 1px #E4E7ED;
    border-radius: 2px;
  }

  &-right {
    display: flex;
    align-items: center;
    column-gap: 6px;
    margin-left: 6px;
  }

  &-divider {
    color: #E4E7ED;
  }
}
</style>
