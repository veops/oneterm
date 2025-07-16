<template>
  <a-modal
    :title="$t('oneterm.assetList.createTempLink')"
    :visible="visible"
    :confirmLoading="confirmLoading"
    :okText="linkText ? $t('confirm') : $t('create')"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <div
      v-if="linkText"
      class="temp-link-content"
    >
      <span class="temp-link-content-label">{{ $t('oneterm.assetList.tempLink') }}: </span>
      <span class="temp-link-content-text">
        {{ linkText }}
        <a @click="copyLink">
          <ops-icon type="veops-copy"/>
        </a>
      </span>
    </div>
    <a-form-model
      v-else
      ref="createTempLinkForm"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 19 }"
    >
      <a-form-model-item :label="$t(`oneterm.assetList.validTime`)" prop="validTime">
        <a-range-picker
          v-model="form.validTime"
          :show-time="{ format: 'HH:mm' }"
          format="YYYY-MM-DD HH:mm"
        >
          <a-icon slot="suffixIcon" type="calendar" />
        </a-range-picker>
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.assetList.times`)" prop="timesRadio">
        <a-radio-group v-model="form.timesRadio">
          <a-radio class="temp-link-times" value="fixed">
            <div class="temp-link-times-fixed">
              <span>{{ $t('oneterm.assetList.fixed') }}</span>
              <a-input-number
                v-model="form.times"
                :min="1"
                :max="9999"
              />
              <span>{{ $t('oneterm.assetList.times2') }}</span>
            </div>
          </a-radio>
          <a-radio class="temp-link-times" value="any">
            {{ $t('oneterm.assetList.any') }}
          </a-radio>
        </a-radio-group>
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import moment from 'moment'
import { postShareLink } from '@/modules/oneterm/api/connect'

export default {
  name: 'ShareAssetModal',
  props: {
    assetData: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      visible: false,
      confirmLoading: false,
      linkText: '',
      form: {
        validTime: [moment(), moment().add(1, 'day')],
        times: 1,
        timesRadio: 'fixed'
      },
      rules: {
        validTime: [
          {
            required: true,
            message: this.$t('placeholder2')
          }
        ],
        timesRadio: [
          {
            required: true,
            message: this.$t('placeholder2')
          }
        ]
      }
    }
  },
  methods: {
    open() {
      this.visible = true
      this.form = {
        validTime: [moment(), moment().add(1, 'day')],
        times: 1,
        timesRadio: 'fixed'
      }
    },
    copyLink() {
      this.$copyText(this.linkText)
        .then(() => {
          this.$message.success(this.$t('copySuccess'))
        })
    },
    handleCancel() {
      this.confirmLoading = false
      this.linkText = ''
      this.visible = false
    },
    handleOk() {
      if (this.linkText) {
        this.handleCancel()
        return
      }

      this.$refs.createTempLinkForm.validate(async (valid) => {
        if (!valid) {
          return
        }
        this.confirmLoading = true
        const start = this.form.validTime[0].format()
        const end = this.form.validTime[1].format()
        const times = this.form.timesRadio === 'fixed' ? this.form.times : 0
        const protocol = this.assetData.protocolType

        const params = [
          {
            account_id: Number(this.assetData.accountId),
            asset_id: this.assetData.assetId,
            protocol,
            start,
            end,
            times,
            no_limit: this.form.timesRadio === 'any'
          }
        ]

        const res = await postShareLink(params)
        const shareId = res?.list?.[0]
        if (shareId) {
          this.$message.success(this.$t('createSuccess'))
          this.linkText = `${document.location.origin}/oneterm/share/${protocol}/${shareId}`
        }
        this.confirmLoading = false
      })
    }
  }
}
</script>

<style lang="less" scoped>
.temp-link-content {
  display: flex;

  &-label {
    flex-shrink: 0;
    margin-right: 12px;
  }

  &-text {
    font-weight: 600;
  }
}

.temp-link-times {
  display: flex;
  align-items: center;

  &-fixed {
    display: flex;
    align-items: center;
    gap: 12px;
  }
}
</style>
