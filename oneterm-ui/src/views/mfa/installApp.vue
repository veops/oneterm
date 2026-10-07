<template>
  <div class="install-app">
    <div class="install-app-title">
      {{ $t('mfa.installApp') }}
    </div>
    <div class="install-app-tip-1">
      {{ $t('mfa.installAppTip1') }}
    </div>

    <div class="install-app-qrcode">
      <div class="install-app-qrcode-col">
        <img class="install-app-qrcode-img" :src="androidDownloadUrl" />
        <span class="install-app-qrcode-text">{{ $t('mfa.androidDownload') }}</span>
      </div>

      <div class="install-app-qrcode-col">
        <img class="install-app-qrcode-img" :src="iphoneDownloadUrl" />
        <span class="install-app-qrcode-text">{{ $t('mfa.iphoneDownload') }}</span>
      </div>
    </div>

    <div class="install-app-tip-2">
      {{ $t('mfa.installAppTip2') }}
    </div>

    <a-button
      type="primary"
      ghost
      class="ops-button-ghost install-app-next"
      @click="clickNext"
    >
      {{ $t('mfa.nextStep') }}
    </a-button>
  </div>
</template>

<script>
import QRCode from 'qrcode'

export default {
  name: 'InstallApp',
  data() {
    return {
      androidDownloadUrl: '',
      iphoneDownloadUrl: ''
    }
  },
  async mounted() {
    const options = {
      margin: 0
    }

    const androidDownloadUrl = await QRCode.toDataURL('https://appgallery.huawei.com/app/C100262999', options)
    const iphoneDownloadUrl = await QRCode.toDataURL('https://apps.apple.com/us/app/microsoft-authenticator/id983156458', options)
    this.androidDownloadUrl = androidDownloadUrl
    this.iphoneDownloadUrl = iphoneDownloadUrl
  },
  methods: {
    clickNext() {
      this.$emit('next')
    }
  }
}
</script>

<style lang="less" scoped>
.install-app {
  width: 100%;
  text-align: center;
  padding: 32px 64px;

  &-title {
    font-size: 18px;
    font-weight: 700;
    color: @text-color_1;
  }

  &-tip-1 {
    margin-top: 24px;
    font-size: 14px;
    font-weight: 700;
    margin-top: 24px;
    color: @text-color_1;
  }

  &-qrcode {
    margin-top: 14px;
    width: 432px;
    background-color: @primary-color_7;
    border-radius: 4px;
    padding: 18px 45px;
    display: flex;
    align-items: center;
    justify-content: space-between;

    &-col {
      display: flex;
      flex-direction: column;
      align-items: center;
    }

    &-img {
      width: 118px;
      height: 118px;
    }

    &-text {
      font-size: 14px;
      font-weight: 400;
      color: @text-color_2;
      margin-top: 6px;
    }
  }

  &-tip-2 {
    margin-top: 24px;
    font-size: 14px;
    font-weight: 700;
    margin-top: 24px;
    color: @text-color_1;
  }

  &-next {
    margin: 24px auto 0;
    width: 328px;
    height: 38px;
    line-height: 38px;
  }
}
</style>
