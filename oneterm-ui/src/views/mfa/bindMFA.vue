<template>
  <div class="bind-mfa">
    <div class="bind-mfa-title">
      {{ $t('mfa.bindMFA') }}
    </div>
    <div class="bind-mfa-tip">
      {{ otp_uri ? $t('mfa.bindMFATip1') : $t('mfa.bindMFATip2') }}
    </div>

    <img
      v-if="otp_uri"
      class="bind-mfa-qrcode"
      :src="qrcodeURL"
    />

    <div v-if="otp_secret_key" class="bind-mfa-secret">
      Secret: {{ otp_secret_key }}
    </div>

    <div class="bind-mfa-code-input">
      <CodeInputBox
        v-model="codeList"
        @enter="submit"
      />
    </div>

    <div v-if="!otp_uri" class="bind-mfa-tip2">
      {{ $t('mfa.bindMFATip3') }}
    </div>

    <a-button
      type="primary"
      ghost
      class="ops-button-ghost bind-mfa-next"
      @click="submit"
    >
      {{ $t('confirm') }}
    </a-button>
  </div>
</template>

<script>
import QRCode from 'qrcode'

import CodeInputBox from '@/components/codeInputBox/index'

export default {
  name: 'BindMFA',
  components: {
    CodeInputBox
  },
  props: {
    otp_uri: {
      type: String,
      default: ''
    },
    otp_secret_key: {
      type: String,
      default: ''
    }
  },
  data() {
    return {
      codeList: ['', '', '', '', '', ''],
      qrcodeURL: ''
    }
  },
  watch: {
    otp_uri: {
      immediate: true,
      deep: true,
      handler(url) {
        if (url) {
          this.initQRCode(url)
        }
      }
    }
  },
  methods: {
    async initQRCode(url) {
      const options = {
        margin: 0
      }

      const qrcodeURL = await QRCode.toDataURL(url, options)
      this.qrcodeURL = qrcodeURL
    },
    async submit() {
      if (this.codeList.some((item) => !item)) {
        this.$message.error(this.$t('mfa.bindMFATip2'))
        return
      }

      this.$emit('submit', this.codeList.join(''))
    }
  }
}
</script>

<style lang="less" scoped>
.bind-mfa {
  width: 100%;
  text-align: center;
  padding: 32px 64px;

  &-title {
    font-size: 18px;
    font-weight: 700;
    color: @text-color_1;
  }

  &-tip {
    margin-top: 24px;
    font-size: 14px;
    font-weight: 700;
    margin-top: 24px;
    color: @text-color_1;
  }

  &-qrcode {
    margin: 14px auto 0;
    width: 131px;
    height: 131px;
  }

  &-secret {
    font-size: 14px;
    font-weight: 400;
    color: @text-color_2;
  }

  &-code-input {
    margin: 14px auto 0;
  }

  &-tip2 {
    margin-top: 34px;
    font-size: 12px;
    color: #86909c;
  }

  &-next {
    margin: 8px auto 0;
    width: 328px;
    max-width: 100%;
    height: 38px;
    line-height: 38px;
  }
}

@media (max-width: 480px) {
  .bind-mfa { padding: 24px 0; }
}
</style>
