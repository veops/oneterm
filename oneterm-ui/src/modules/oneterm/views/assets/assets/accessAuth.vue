<template>
  <a-form-model ref="form" :model="form" :rules="rules" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
    <a-form-model-item>
      <span slot="label">
        <a-tooltip placement="right" :title="$t('oneterm.assetList.timeTip')">
          <a><a-icon type="question-circle"/></a>
        </a-tooltip>
        {{ $t(`oneterm.assetList.time`) }}
      </span>
      <a-radio-group v-model="form.allow" style="display:block;margin:8px 0;">
        <a-radio :value="true">
          {{ $t('oneterm.assetList.allowAccess') }}
        </a-radio>
        <a-radio :value="false">
          {{ $t('oneterm.assetList.prohibitAccess') }}
        </a-radio>
      </a-radio-group>
      <DragWeektime v-model="form.ranges" :data="weektimeData" @onClear="clearWeektime" />
    </a-form-model-item>
    <a-form-model-item :label="$t(`oneterm.assetList.effectiveDate`)" prop="startAndEnd">
      <a-range-picker v-model="form.startAndEnd" />
    </a-form-model-item>
    <a-form-model-item :label="$t(`oneterm.assetList.commandIntercept`)" prop="cmd_ids">
      <treeselect
        class="custom-treeselect custom-treeselect-white"
        :style="{
          '--custom-height': '32px',
          lineHeight: '32px',
          '--custom-multiple-lineHeight': '18px',
        }"
        v-model="form.cmd_ids"
        :multiple="true"
        :clearable="true"
        searchable
        :options="cmdList"
        :placeholder="`${$t(`placeholder2`)}`"
        :normalizer="
          (node) => {
            return {
              id: node.id,
              label: node.name,
            }
          }
        "
        appendToBody
        :z-index="1056"
      >
      </treeselect>
    </a-form-model-item>
  </a-form-model>
</template>

<script>
import moment from 'moment'
import { getCommandList } from '../../../api/command'
import DragWeektime from '../../../components/dragWeektime'
import weektimeData from '../../../components/dragWeektime/weektimeData'

export default {
  name: 'AccessAuth',
  components: { DragWeektime },
  data() {
    return {
      weektimeData,
      weekMap: {
        0: '一',
        1: '二',
        2: '三',
        3: '四',
        4: '五',
        5: '六',
        6: '七',
      },
      form: {
        cmd_ids: undefined,
        startAndEnd: [],
        ranges: [],
        allow: true,
      },
      rules: {},
      allRoleList: [],
      roleList: [],
      cmdList: [],
    }
  },
  created() {
    getCommandList({ page_index: 1 }).then((res) => {
      this.cmdList = res?.data?.list || []
    })
    this.form.ranges = this.weektimeData.map((item) => {
      return {
        id: item.row,
        week: item.week,
        value: [],
      }
    })
  },
  beforeDestroy() {
    this.clearWeektime()
  },
  methods: {
    clearWeektime() {
      this.weektimeData.forEach((item) => {
        item.child.forEach((t) => {
          this.$set(t, 'check', false)
        })
      })
      this.form.ranges = this.weektimeData.map((item) => {
        return {
          id: item.row,
          week: item.week,
          value: [],
        }
      })
    },
    getValues() {
      const { cmd_ids, startAndEnd, ranges, allow } = this.form
      return {
        cmd_ids,
        start: startAndEnd[0]
          ? moment(startAndEnd[0])
              .startOf('day')
              .format()
          : null,
        end: startAndEnd[1]
          ? moment(startAndEnd[1])
              .endOf('day')
              .format()
          : null,
        ranges: ranges.map((r) => ({
          week: r.id,
          times: r.value,
        })),
        allow,
      }
    },
    async setValues(access_auth) {
      const { cmd_ids = undefined, start, end, ranges = [], allow = true } = access_auth
      this.form = {
        cmd_ids,
        allow,
        startAndEnd: [start ? moment(start) : null, end ? moment(end) : null],
        ranges: ranges.map((r, index) => {
          this.weektimeData[index].child.forEach((t) => {
            this.$set(t, 'check', !!r.times && r.times.includes(t.value))
          })
          return {
            id: index,
            week: `星期${this.weekMap[index]}`,
            value: r.times,
          }
        }),
      }
      if (!ranges.length) {
        this.clearWeektime()
      }
    },
  },
}
</script>

<style lang="less">
.access-auth-user {
  .ant-form-item-control {
    line-height: 32px;
  }
  .vue-treeselect__multi-value {
    line-height: 18px;
  }
}
</style>
