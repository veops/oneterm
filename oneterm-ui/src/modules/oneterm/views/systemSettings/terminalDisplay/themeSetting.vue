<template>
  <div class="theme-setting">
    <div
      v-for="(item) in themeList"
      :key="item.key"
      :class="[
        'theme-setting-item',
        currentTheme === item.key ? 'theme-setting-item-active' : ''
      ]"
      @click="clickTheme(item.key)"
    >
      <div
        class="theme-setting-item-container"
        :style="{
          backgroundColor: item.themeObj.background || '#FFFFFF'
        }"
      >
        <div class="theme-setting-item-row">
          <div
            class="theme-setting-item-circle"
            :style="{
              backgroundColor: item.themeObj.brightBlack
            }"
          ></div>
          <div
            class="theme-setting-item-bar"
            :style="{
              backgroundColor: item.themeObj.brightBlack,
              width: '55px'
            }"
          ></div>
          <div class="theme-setting-item-text">
            <span
              :style="{
                color: item.themeObj.brightGreen
              }"
            >OneTerm</span>
            <span
              :style="{
                color: item.themeObj.brightBlue,
                marginLeft: '6px'
              }"
            >Text Text</span>
          </div>
        </div>

        <div class="theme-setting-item-row">
          <div
            class="theme-setting-item-circle"
            :style="{
              backgroundColor: item.themeObj.brightBlack
            }"
          ></div>
          <div
            class="theme-setting-item-bar"
            :style="{
              backgroundColor: item.themeObj.brightBlack,
              width: '92px'
            }"
          ></div>
          <div
            class="theme-setting-item-text"
            :style="{
              color: item.themeObj.foreground
            }"
          >
            Text Text Text
          </div>
        </div>
      </div>

      <div class="theme-setting-item-name">
        {{ item.key }}
      </div>

      <div
        v-show="currentTheme === item.key"
        class="theme-setting-item-active-check"
      >
        <a-icon type="check" class="theme-setting-item-active-check-icon" />
      </div>
    </div>
  </div>
</template>

<script>
import XtermTheme from 'xterm-theme'
import { defaultPreferenceSetting } from './constants.js'
import { getPreference, putPreference } from '@/modules/oneterm/api/preference.js'

export default {
  name: 'ThemeSetting',
  data() {
    return {
      currentTheme: defaultPreferenceSetting.theme,
      isLoading: false
    }
  },
  computed: {
    themeList() {
      return Object.keys(XtermTheme).map((key) => {
        return {
          key,
          themeObj: XtermTheme?.[key] || {}
        }
      })
    }
  },
  mounted() {
    this.initData()
  },
  methods: {
    async initData() {
      const res = await getPreference()
      const data = res?.data || {}

      this.currentTheme = data?.theme || defaultPreferenceSetting.theme
    },

    async clickTheme(key) {
      if (key === this.currentTheme || this.isLoading) {
        return
      }

      this.isLoading = true
      const oldTheme = this.currentTheme
      this.currentTheme = key

      putPreference({
        theme: this.currentTheme
      }).then(() => {
        this.$emit('ok')
      }).catch(() => {
        this.currentTheme = oldTheme
      }).finally(() => {
        this.isLoading = false
      })
    }
  }
}
</script>

<style lang="less" scoped>
.theme-setting {
  width: 100%;
  height: calc(100vh - 130px);
  padding: 18px;
  background-color: #FFFFFF;
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
  flex-wrap: wrap;
  row-gap: 15px;
  column-gap: 30px;

  &-item {
    padding: 16px;
    border: solid 1px @text-color_6;
    background-color: #FFFFFF;
    cursor: pointer;
    min-width: 320px;
    border-radius: 4px;
    position: relative;

    &-container {
      padding: 16px;
      border-radius: 4px;
    }

    &-row {
      display: flex;
      align-items: center;
    }

    &-circle {
      width: 14px;
      height: 14px;
      border-radius: 14px;
    }

    &-bar {
      margin-left: 10px;
      height: 12px;
      border-radius: 2px;
    }

    &-text {
      margin-left: auto;
      font-size: 14px;
      line-height: 24px;
      color: #FFFFFF;
    }

    &-name {
      font-size: 14px;
      line-height: 24px;
      margin-top: 8px;
      color: @text-color_1;
    }

    &-active {
      border-color: @primary-color_8;

      &-check {
        position: absolute;
        bottom: 0;
        right: 0;
        z-index: 4;
        width: 36px;
        height: 25px;
        border-left: 36px solid transparent;
        border-bottom: 25px solid @primary-color;

        &-icon {
          font-size: 12px;
          color: #FFFFFF;
          position: absolute;
          top: 10px;
          right: 3px;
        }
      }
    }

    &:hover {
      border-color: @primary-color_8;
    }
  }
}
</style>
