<template>
  <a-form-model
    ref="createTempLinkForm"
    :model="form"
    :rules="rules"
    :label-col="{ span: 5 }"
    :wrapper-col="{ span: 19 }"
  >
    <a-form-model-item :label="$t(`oneterm.assetList.account`)" prop="accountIds">
      <div class="temp-link-account">
        <div
          v-for="(account) in accountList"
          :key="account.key"
          :class="['temp-link-account-item', form.accountIds.includes(account.key) ? 'temp-link-account-item-active' : '']"
          @click="clickAccount(account.key)"
        >
          <ops-icon
            :type="account.protocolIcon"
            class="temp-link-account-item-icon"
          />
          <span
            class="temp-link-account-item-name"
          >
            {{ account.account_name }}
          </span>
        </div>
      </div>
    </a-form-model-item>
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
    <div class="temp-link-operation">
      <a-button @click="handleCancel('cancel')">
        {{ $t('cancel') }}
      </a-button>

      <a-button
        type="primary"
        @click="handleOk"
      >
        {{ $t('confirm') }}
      </a-button>
    </div>
  </a-form-model>
</template>

<script>
import { postShareLink } from '@/modules/oneterm/api/connect'

export default {
  name: 'CreateTempLink',
  props: {
    assetData: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      accountList: [],
      form: {
        accountIds: [],
        validTime: null,
        times: 1,
        timesRadio: 'fixed'
      },
      rules: {
        accountIds: [
          {
            validator: (rule, value, callback) => {
              if (!value?.length) {
                callback(new Error(this.$t('placeholder2')))
              } else {
                callback()
              }
            },
            trigger: 'change'
          }
        ],
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
  mounted() {
    this.accountList = this?.assetData?.accountList?.map?.((item) => {
      return {
        ...item,
        key: `${item.protocol}&${item.account_id}`
      }
    }) || []
  },
  methods: {
    handleCancel(eventEmit) {
      this.form = {
        accountIds: [],
        validTime: null,
        times: 1,
        timesRadio: 'fixed'
      }
      this.$refs.createTempLinkForm.clearValidate()

      if (eventEmit) {
        this.$emit(eventEmit)
      }
    },
    handleOk() {
      this.$refs.createTempLinkForm.validate(async (valid) => {
        if (!valid) {
          return
        }

        const start = this.form.validTime[0].format()
        const end = this.form.validTime[1].format()
        const times = this.form.timesRadio === 'fixed' ? this.form.times : 0

        const data = this.form.accountIds.map((accountKey) => {
          const [protocol, account_id] = accountKey.split('&')

          return {
            account_id: Number(account_id),
            asset_id: this?.assetData?.id,
            start,
            end,
            protocol,
            times,
            no_limit: this.form.timesRadio === 'any'
          }
        })

        const res = await postShareLink(data)
        const linkList = res?.list

        if (linkList) {
          this.$message.success(this.$t('createSuccess'))
          this.handleCancel('ok')
        }
      })
    },
    clickAccount(id) {
      const index = this.form.accountIds.findIndex((item) => item === id)
      if (index > -1) {
        this.form.accountIds.splice(index, 1)
      } else {
        this.form.accountIds.push(id)
      }
    },
  }
}
</script>

<style lang="less" scoped>
.temp-link-account {
  display: flex;
  flex-wrap: wrap;
  column-gap: 36px;
  row-gap: 18px;
  max-height: 300px;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 20px;

  &-item {
    flex-shrink: 0;
    padding: 4px 8px;
    display: flex;
    align-items: center;
    cursor: pointer;
    border: solid 2px transparent;
    background-color: #F7F8FA;
    line-height: 22px;
    border-radius: 2px;
    max-width: 100%;

    &-icon {
      font-size: 14px;
      color: #2F54EB;
      margin-right: 8px;
    }

    &-name {
      font-size: 14px;
      font-weight: 400;
      color: #1D2129;
      word-break: break-all;
    }

    &-active {
      border-color: #7F97FA;
      background-color: #E1EFFF;
    }
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

.temp-link-operation {
  display: flex;
  justify-content: flex-end;
  column-gap: 8px;
}
</style>
